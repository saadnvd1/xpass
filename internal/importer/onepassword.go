package importer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/saadnvd1/xpass/internal/vault"
)

// ImportResult tracks what happened during an import
type ImportResult struct {
	Total    int
	Imported int
	Skipped  int
	Errors   []string
	Entries  []vault.Entry
}

// Import auto-detects format and imports from 1Password export file
func Import(path string) (*ImportResult, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".csv":
		return ImportCSV(path)
	case ".json":
		return ImportJSON(path)
	default:
		// Try to detect by content
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		content := strings.TrimSpace(string(data))
		if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
			return importJSONData(data)
		}
		return importCSVData(data)
	}
}

// ImportCSV imports from 1Password CSV export
func ImportCSV(path string) (*ImportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return importCSVData(data)
}

func importCSVData(data []byte) (*ImportResult, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	// Read header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	// Normalize headers
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	result := &ImportResult{}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", result.Total+1, err))
			result.Skipped++
			result.Total++
			continue
		}

		result.Total++

		entry, err := parseCSVRow(headerMap, record)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", result.Total, err))
			result.Skipped++
			continue
		}
		if entry == nil {
			result.Skipped++
			continue
		}

		result.Entries = append(result.Entries, *entry)
		result.Imported++
	}

	return result, nil
}

func getField(headerMap map[string]int, record []string, names ...string) string {
	for _, name := range names {
		if idx, ok := headerMap[name]; ok && idx < len(record) {
			return strings.TrimSpace(record[idx])
		}
	}
	return ""
}

func parseCSVRow(headerMap map[string]int, record []string) (*vault.Entry, error) {
	title := getField(headerMap, record, "title", "name")
	if title == "" {
		title = "Untitled"
	}

	username := getField(headerMap, record, "username", "user")
	password := getField(headerMap, record, "password", "pass")
	url := getField(headerMap, record, "url", "website", "login_uri")
	notes := getField(headerMap, record, "notes", "notesplain")
	typeName := getField(headerMap, record, "type", "category")
	otpAuth := getField(headerMap, record, "otpauth", "otp", "one-time password")

	now := time.Now().UTC().Format(time.RFC3339)

	entry := &vault.Entry{
		Name:      title,
		Tags:      []string{"1password-import"},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
		Notes:     notes,
	}

	// Detect type
	entryType := detectCSVType(typeName, username, password, notes)
	entry.Type = entryType

	switch entryType {
	case vault.TypeLogin:
		entry.Username = username
		entry.Password = password
		entry.URL = url
		if otpAuth != "" && strings.HasPrefix(otpAuth, "otpauth://") {
			entry.TOTP = parseTOTPUri(otpAuth)
		}
	case vault.TypeSecureNote:
		entry.Content = notes
	}

	// Skip empty entries
	if entry.Password == "" && entry.Username == "" && entry.Content == "" && entry.Notes == "" {
		return nil, nil
	}

	return entry, nil
}

func detectCSVType(typeName, username, password, notes string) vault.EntryType {
	t := strings.ToLower(typeName)

	if strings.Contains(t, "note") {
		return vault.TypeSecureNote
	}
	if strings.Contains(t, "credit") || strings.Contains(t, "card") {
		return vault.TypeCreditCard
	}
	if strings.Contains(t, "identity") {
		return vault.TypeIdentity
	}
	if strings.Contains(t, "ssh") {
		return vault.TypeSSHKey
	}
	if strings.Contains(t, "api") {
		return vault.TypeAPIKey
	}

	if password != "" || username != "" {
		return vault.TypeLogin
	}
	if notes != "" {
		return vault.TypeSecureNote
	}
	return vault.TypeLogin
}

// ImportJSON imports from 1Password JSON export
func ImportJSON(path string) (*ImportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return importJSONData(data)
}

// 1Password JSON structures
type opItem struct {
	UUID      string         `json:"uuid"`
	Title     string         `json:"title"`
	Category  string         `json:"category"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
	URLs      []opURL        `json:"urls"`
	Fields    []opField      `json:"fields"`
	Sections  []opSection    `json:"sections"`
	Notes     string         `json:"notes"`
	Tags      []string       `json:"tags"`
	Favorite  int            `json:"favorite"`
}

type opURL struct {
	Href    string `json:"href"`
	Primary bool   `json:"primary"`
}

type opField struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Designation string `json:"designation"`
}

type opSection struct {
	Title  string          `json:"title"`
	Fields []opSectionField `json:"fields"`
}

type opSectionField struct {
	Title string      `json:"title"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

type opExport struct {
	Accounts []struct {
		Vaults []struct {
			Items []opItem `json:"items"`
		} `json:"vaults"`
	} `json:"accounts"`
	Vaults []struct {
		Items []opItem `json:"items"`
	} `json:"vaults"`
	Items []opItem `json:"items"`
}

func importJSONData(data []byte) (*ImportResult, error) {
	items := extractItems(data)

	result := &ImportResult{Total: len(items)}

	for i, item := range items {
		entry, err := parseJSONItem(item)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("item %d: %v", i+1, err))
			result.Skipped++
			continue
		}
		if entry == nil {
			result.Skipped++
			continue
		}

		result.Entries = append(result.Entries, *entry)
		result.Imported++
	}

	return result, nil
}

func extractItems(data []byte) []opItem {
	// Try as array of items
	var items []opItem
	if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
		return items
	}

	// Try as export object
	var export opExport
	if err := json.Unmarshal(data, &export); err == nil {
		// Direct items
		if len(export.Items) > 0 {
			return export.Items
		}
		// Vaults
		for _, v := range export.Vaults {
			items = append(items, v.Items...)
		}
		// Accounts > Vaults
		for _, a := range export.Accounts {
			for _, v := range a.Vaults {
				items = append(items, v.Items...)
			}
		}
		if len(items) > 0 {
			return items
		}
	}

	// Try as single item
	var single opItem
	if err := json.Unmarshal(data, &single); err == nil && single.Title != "" {
		return []opItem{single}
	}

	return nil
}

func parseJSONItem(item opItem) (*vault.Entry, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	updatedAt := createdAt
	if item.CreatedAt > 0 {
		createdAt = time.Unix(item.CreatedAt, 0).UTC().Format(time.RFC3339)
	}
	if item.UpdatedAt > 0 {
		updatedAt = time.Unix(item.UpdatedAt, 0).UTC().Format(time.RFC3339)
	}

	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
	tags = append(tags, "1password-import")

	entry := &vault.Entry{
		Name:      item.Title,
		Tags:      tags,
		Favorite:  item.Favorite == 1,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Version:   1,
		Notes:     item.Notes,
	}

	if entry.Name == "" {
		entry.Name = "Untitled"
	}

	category := strings.ToLower(item.Category)

	switch category {
	case "login", "website", "server", "":
		entry.Type = vault.TypeLogin
		parseLoginFields(entry, item)

	case "securenote", "secure note", "note", "document":
		entry.Type = vault.TypeSecureNote
		entry.Content = item.Notes
		// Append section data to content
		for _, s := range item.Sections {
			for _, f := range s.Fields {
				val := sectionFieldValue(f)
				if val != "" {
					entry.Content += "\n" + f.Title + ": " + val
				}
			}
		}

	case "credit card", "creditcard", "credit_card":
		entry.Type = vault.TypeCreditCard
		parseCreditCardFields(entry, item)

	case "identity":
		entry.Type = vault.TypeIdentity
		parseIdentityFields(entry, item)

	default:
		// Try login if has password field
		hasPassword := false
		for _, f := range item.Fields {
			if strings.ToLower(f.Designation) == "password" || strings.ToLower(f.Name) == "password" {
				hasPassword = true
				break
			}
		}
		if hasPassword {
			entry.Type = vault.TypeLogin
			parseLoginFields(entry, item)
		} else if item.Notes != "" {
			entry.Type = vault.TypeSecureNote
			entry.Content = item.Notes
		} else {
			return nil, nil
		}
	}

	return entry, nil
}

func parseLoginFields(entry *vault.Entry, item opItem) {
	// Extract URL
	for _, u := range item.URLs {
		if u.Primary || entry.URL == "" {
			entry.URL = u.Href
		}
	}

	// Extract from fields
	for _, f := range item.Fields {
		designation := strings.ToLower(f.Designation)
		name := strings.ToLower(f.Name)

		if designation == "username" || name == "username" {
			entry.Username = f.Value
		} else if designation == "password" || name == "password" {
			entry.Password = f.Value
		} else if f.Type == "otp" || name == "one-time password" {
			if strings.HasPrefix(f.Value, "otpauth://") {
				entry.TOTP = parseTOTPUri(f.Value)
			} else if f.Value != "" {
				entry.TOTP = &vault.TOTP{Secret: f.Value, Algorithm: "SHA1", Digits: 6, Period: 30}
			}
		}
	}

	// Extract from sections
	for _, s := range item.Sections {
		for _, f := range s.Fields {
			title := strings.ToLower(f.Title)
			val := sectionFieldValue(f)

			if strings.Contains(title, "username") {
				entry.Username = val
			} else if strings.Contains(title, "password") {
				entry.Password = val
			} else if strings.Contains(title, "email") {
				entry.Email = val
			}
		}
	}
}

func parseCreditCardFields(entry *vault.Entry, item opItem) {
	for _, f := range item.Fields {
		name := strings.ToLower(f.Name)
		if strings.Contains(name, "cardholder") || strings.Contains(name, "holder") {
			entry.CardholderName = f.Value
		} else if strings.Contains(name, "number") || name == "card" {
			entry.CardNumber = f.Value
		} else if strings.Contains(name, "cvv") || strings.Contains(name, "cvc") {
			entry.CVV = f.Value
		} else if strings.Contains(name, "pin") {
			entry.PIN = f.Value
		} else if strings.Contains(name, "expir") {
			parts := strings.Split(f.Value, "/")
			if len(parts) == 2 {
				entry.ExpiryMonth = strings.TrimSpace(parts[0])
				entry.ExpiryYear = strings.TrimSpace(parts[1])
			}
		}
	}
}

func parseIdentityFields(entry *vault.Entry, item opItem) {
	for _, f := range item.Fields {
		name := strings.ToLower(f.Name)
		if strings.Contains(name, "first") {
			entry.Username = f.Value // reuse for display
		} else if strings.Contains(name, "email") {
			entry.Email = f.Value
		}
	}
}

func sectionFieldValue(f opSectionField) string {
	switch v := f.Value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if concealed, ok := v["concealed"]; ok {
			return fmt.Sprintf("%v", concealed)
		}
		if totp, ok := v["totp"]; ok {
			return fmt.Sprintf("%v", totp)
		}
	}
	return fmt.Sprintf("%v", f.Value)
}

func parseTOTPUri(uri string) *vault.TOTP {
	// Basic otpauth:// parser
	// Format: otpauth://totp/Label?secret=XXX&algorithm=SHA1&digits=6&period=30
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		return nil
	}

	totp := &vault.TOTP{
		Algorithm: "SHA1",
		Digits:    6,
		Period:    30,
	}

	parts := strings.SplitN(uri, "?", 2)
	if len(parts) < 2 {
		return nil
	}

	for _, param := range strings.Split(parts[1], "&") {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "secret":
			totp.Secret = kv[1]
		case "algorithm":
			totp.Algorithm = strings.ToUpper(kv[1])
		case "digits":
			fmt.Sscanf(kv[1], "%d", &totp.Digits)
		case "period":
			fmt.Sscanf(kv[1], "%d", &totp.Period)
		}
	}

	if totp.Secret == "" {
		return nil
	}
	return totp
}
