package importer

import (
	"archive/zip"
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
	case ".1pux":
		return Import1PUX(path)
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

// Import1PUX imports from 1Password .1pux file (ZIP containing export.data JSON)
func Import1PUX(path string) (*ImportResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("opening 1pux file: %w", err)
	}
	defer r.Close()

	// Find export.data inside the ZIP
	for _, f := range r.File {
		if f.Name == "export.data" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("reading export.data: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("reading export.data: %w", err)
			}

			return import1PUXData(data)
		}
	}

	return nil, fmt.Errorf("invalid 1pux file: missing export.data")
}

// 1PUX-specific structures (different from standard JSON export)
type puxExport struct {
	Accounts []puxAccount `json:"accounts"`
}

type puxAccount struct {
	Attrs  puxAttrs   `json:"attrs"`
	Vaults []puxVault `json:"vaults"`
}

type puxAttrs struct {
	AccountName string `json:"accountName"`
	Email       string `json:"email"`
}

type puxVault struct {
	Attrs puxVaultAttrs `json:"attrs"`
	Items []puxItem     `json:"items"`
}

type puxVaultAttrs struct {
	Name string `json:"name"`
}

type puxItem struct {
	UUID         string       `json:"uuid"`
	FavIndex     int          `json:"favIndex"`
	CreatedAt    int64        `json:"createdAt"`
	UpdatedAt    int64        `json:"updatedAt"`
	CategoryUUID string       `json:"categoryUuid"`
	Overview     puxOverview  `json:"overview"`
	Details      puxDetails   `json:"details"`
}

type puxOverview struct {
	Title string   `json:"title"`
	URLs  []puxURL `json:"urls"`
	URL   string   `json:"url"`
	Tags  []string `json:"tags"`
}

type puxURL struct {
	URL string `json:"url"`
}

type puxDetails struct {
	LoginFields []puxLoginField `json:"loginFields"`
	NotesPlain  string          `json:"notesPlain"`
	Sections    []puxSection    `json:"sections"`
}

type puxLoginField struct {
	Designation string `json:"designation"`
	Value       string `json:"value"`
	Name        string `json:"name"`
	Type        string `json:"type"`
}

type puxSection struct {
	Title  string         `json:"title"`
	Fields []puxSectField `json:"fields"`
}

type puxSectField struct {
	Title string      `json:"title"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func import1PUXData(data []byte) (*ImportResult, error) {
	var export puxExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parsing export.data: %w", err)
	}

	// Collect all items from all accounts/vaults
	var allItems []puxItem
	for _, account := range export.Accounts {
		for _, v := range account.Vaults {
			allItems = append(allItems, v.Items...)
		}
	}

	result := &ImportResult{Total: len(allItems)}

	for i, item := range allItems {
		entry, err := parsePUXItem(item)
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

func parsePUXItem(item puxItem) (*vault.Entry, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	updatedAt := createdAt
	if item.CreatedAt > 0 {
		createdAt = time.Unix(item.CreatedAt, 0).UTC().Format(time.RFC3339)
	}
	if item.UpdatedAt > 0 {
		updatedAt = time.Unix(item.UpdatedAt, 0).UTC().Format(time.RFC3339)
	}

	tags := item.Overview.Tags
	if tags == nil {
		tags = []string{}
	}
	tags = append(tags, "1password-import")

	entry := &vault.Entry{
		Name:      item.Overview.Title,
		Tags:      tags,
		Favorite:  item.FavIndex > 0,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Version:   1,
		Notes:     item.Details.NotesPlain,
	}

	if entry.Name == "" {
		entry.Name = "Untitled"
	}

	// Category UUIDs: 001=Login, 002=CreditCard, 003=SecureNote, 004=Identity, 005=Password, 006=Document
	switch item.CategoryUUID {
	case "001", "005", "": // Login or Password
		entry.Type = vault.TypeLogin

		// URL
		if len(item.Overview.URLs) > 0 {
			entry.URL = item.Overview.URLs[0].URL
		} else if item.Overview.URL != "" {
			entry.URL = item.Overview.URL
		}

		// Login fields
		for _, f := range item.Details.LoginFields {
			switch strings.ToLower(f.Designation) {
			case "username":
				entry.Username = f.Value
			case "password":
				entry.Password = f.Value
			}
		}

		// Check sections for OTP and other fields
		for _, s := range item.Details.Sections {
			for _, f := range s.Fields {
				title := strings.ToLower(f.Title)
				val := puxFieldValue(f)

				if strings.Contains(title, "one-time") || strings.Contains(title, "otp") || strings.Contains(title, "2fa") {
					if strings.HasPrefix(val, "otpauth://") {
						entry.TOTP = parseTOTPUri(val)
					} else if val != "" {
						entry.TOTP = &vault.TOTP{Secret: val, Algorithm: "SHA1", Digits: 6, Period: 30}
					}
				} else if strings.Contains(title, "email") {
					entry.Email = val
				}
			}
		}

		// Also check if TOTP is embedded in value as map with "totp" key
		for _, s := range item.Details.Sections {
			for _, f := range s.Fields {
				if m, ok := f.Value.(map[string]interface{}); ok {
					if totp, ok := m["totp"]; ok {
						secret := fmt.Sprintf("%v", totp)
						if secret != "" && entry.TOTP == nil {
							entry.TOTP = &vault.TOTP{Secret: secret, Algorithm: "SHA1", Digits: 6, Period: 30}
						}
					}
				}
			}
		}

	case "002": // Credit Card
		entry.Type = vault.TypeCreditCard
		for _, s := range item.Details.Sections {
			for _, f := range s.Fields {
				title := strings.ToLower(f.Title)
				val := puxFieldValue(f)

				if strings.Contains(title, "cardholder") {
					entry.CardholderName = val
				} else if strings.Contains(title, "number") || title == "card number" {
					entry.CardNumber = val
				} else if strings.Contains(title, "cvv") || strings.Contains(title, "verification") {
					entry.CVV = val
				} else if strings.Contains(title, "pin") {
					entry.PIN = val
				} else if strings.Contains(title, "expir") {
					// Try monthYear format
					if m, ok := f.Value.(map[string]interface{}); ok {
						if my, ok := m["monthYear"]; ok {
							parts := strings.Split(fmt.Sprintf("%v", my), "/")
							if len(parts) == 2 {
								entry.ExpiryMonth = parts[0]
								entry.ExpiryYear = parts[1]
							}
						}
					}
				}
			}
		}

	case "003": // Secure Note
		entry.Type = vault.TypeSecureNote
		entry.Content = item.Details.NotesPlain

	case "004": // Identity
		entry.Type = vault.TypeIdentity
		for _, s := range item.Details.Sections {
			for _, f := range s.Fields {
				title := strings.ToLower(f.Title)
				val := puxFieldValue(f)

				if strings.Contains(title, "first") {
					entry.Username = val
				} else if strings.Contains(title, "email") {
					entry.Email = val
				}
			}
		}

	case "006": // Document — store as secure note
		entry.Type = vault.TypeSecureNote
		entry.Content = item.Details.NotesPlain
		if entry.Content == "" && entry.Name == "Untitled" {
			return nil, nil
		}

	default:
		// Unknown — try secure note
		if item.Details.NotesPlain != "" || entry.Name != "Untitled" {
			entry.Type = vault.TypeSecureNote
			entry.Content = item.Details.NotesPlain
		} else {
			return nil, nil
		}
	}

	return entry, nil
}

func puxFieldValue(f puxSectField) string {
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
