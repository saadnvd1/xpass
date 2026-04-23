package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/saadnvd1/xpass/internal/crypto"
)

const (
	VaultFile   = "vault.json"
	ConfigFile  = "config.json"
	SessionFile = ".session"
)

// Vault manages the encrypted credential store
type Vault struct {
	dir      string
	entries  []Entry
	config   Config
	password string
}

// New creates a vault instance pointing at dir
func New(dir string) *Vault {
	return &Vault{dir: dir}
}

// DefaultDir returns ~/.xpass
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".xpass")
}

// Dir returns the vault directory path
func (v *Vault) Dir() string {
	return v.dir
}

// Exists checks if a vault has been initialized
func (v *Vault) Exists() bool {
	_, err := os.Stat(filepath.Join(v.dir, VaultFile))
	return err == nil
}

// Init creates a new vault with the given master password
func (v *Vault) Init(password string) error {
	if v.Exists() {
		return fmt.Errorf("vault already exists at %s", v.dir)
	}

	if err := os.MkdirAll(v.dir, 0700); err != nil {
		return fmt.Errorf("creating vault dir: %w", err)
	}

	v.entries = []Entry{}
	v.config = DefaultConfig()
	v.password = password

	if err := v.save(); err != nil {
		return fmt.Errorf("saving vault: %w", err)
	}

	// Write .gitignore
	gitignore := ".session\n*.bak\n*.tmp\n"
	os.WriteFile(filepath.Join(v.dir, ".gitignore"), []byte(gitignore), 0600)

	return nil
}

// Unlock decrypts the vault with the master password
func (v *Vault) Unlock(password string) error {
	if !v.Exists() {
		return fmt.Errorf("no vault at %s — run init first", v.dir)
	}

	// Decrypt vault entries
	entries, err := v.decryptFile(VaultFile, password)
	if err != nil {
		return fmt.Errorf("invalid password or corrupted vault")
	}

	var parsed []Entry
	if err := json.Unmarshal([]byte(entries), &parsed); err != nil {
		return fmt.Errorf("parsing vault data: %w", err)
	}

	// Decrypt config
	configData, err := v.decryptFile(ConfigFile, password)
	if err == nil {
		json.Unmarshal([]byte(configData), &v.config)
	} else {
		v.config = DefaultConfig()
	}

	v.entries = parsed
	v.password = password
	return nil
}

// Lock clears decrypted data from memory
func (v *Vault) Lock() {
	v.entries = nil
	v.password = ""
}

// IsUnlocked returns whether the vault is currently decrypted
func (v *Vault) IsUnlocked() bool {
	return v.password != ""
}

// Entries returns all entries
func (v *Vault) Entries() []Entry {
	return v.entries
}

// Count returns entry count
func (v *Vault) Count() int {
	return len(v.entries)
}

// Config returns vault config
func (v *Vault) Config() Config {
	return v.config
}

// Add creates a new entry and saves
func (v *Vault) Add(entry Entry) (*Entry, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	id, err := crypto.GenerateID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	entry.ID = id
	entry.CreatedAt = now
	entry.UpdatedAt = now
	entry.Version = 1
	if entry.Tags == nil {
		entry.Tags = []string{}
	}

	v.entries = append(v.entries, entry)

	if err := v.save(); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Update modifies an existing entry
func (v *Vault) Update(id string, updates Entry) (*Entry, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}

	for i, e := range v.entries {
		if e.ID == id {
			updates.ID = e.ID
			updates.CreatedAt = e.CreatedAt
			updates.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			updates.Version = e.Version + 1
			v.entries[i] = updates
			if err := v.save(); err != nil {
				return nil, err
			}
			return &v.entries[i], nil
		}
	}
	return nil, fmt.Errorf("entry not found: %s", id)
}

// Delete removes an entry by ID
func (v *Vault) Delete(id string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}

	for i, e := range v.entries {
		if e.ID == id {
			v.entries = append(v.entries[:i], v.entries[i+1:]...)
			return v.save()
		}
	}
	return fmt.Errorf("entry not found: %s", id)
}

// Get returns an entry by ID
func (v *Vault) Get(id string) *Entry {
	for i, e := range v.entries {
		if e.ID == id {
			return &v.entries[i]
		}
	}
	return nil
}

// GetByName finds entry by exact name (case-insensitive)
func (v *Vault) GetByName(name string) *Entry {
	lower := strings.ToLower(name)
	for i, e := range v.entries {
		if strings.ToLower(e.Name) == lower {
			return &v.entries[i]
		}
	}
	return nil
}

// Search performs fuzzy text matching across entries
func (v *Vault) Search(query string) []Entry {
	if query == "" {
		return v.entries
	}

	query = strings.ToLower(query)
	var results []Entry

	for _, e := range v.entries {
		searchable := strings.ToLower(
			e.Name + " " + e.Username + " " + e.Email + " " + e.URL + " " +
				strings.Join(e.Tags, " ") + " " + e.Notes + " " + e.Content,
		)
		if strings.Contains(searchable, query) {
			results = append(results, e)
		}
	}
	return results
}

// Tags returns all unique tags
func (v *Vault) Tags() []string {
	seen := map[string]bool{}
	var tags []string
	for _, e := range v.entries {
		for _, t := range e.Tags {
			if !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
	}
	return tags
}

// Favorites returns favorited entries
func (v *Vault) Favorites() []Entry {
	var fav []Entry
	for _, e := range v.entries {
		if e.Favorite {
			fav = append(fav, e)
		}
	}
	return fav
}

// TrackAccess updates access metadata for an entry
func (v *Vault) TrackAccess(id string) {
	for i, e := range v.entries {
		if e.ID == id {
			v.entries[i].LastAccessed = time.Now().UTC().Format(time.RFC3339)
			v.entries[i].AccessCount++
			v.save()
			return
		}
	}
}

// save encrypts and writes vault + config to disk
func (v *Vault) save() error {
	// Save entries
	data, err := json.Marshal(v.entries)
	if err != nil {
		return err
	}
	if err := v.encryptToFile(VaultFile, string(data)); err != nil {
		return err
	}

	// Save config
	configData, err := json.Marshal(v.config)
	if err != nil {
		return err
	}
	return v.encryptToFile(ConfigFile, string(configData))
}

func (v *Vault) encryptToFile(filename, plaintext string) error {
	encrypted, err := crypto.Encrypt(plaintext, v.password)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(encrypted, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(v.dir, filename), data, 0600)
}

func (v *Vault) decryptFile(filename, password string) (string, error) {
	path := filepath.Join(v.dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var encrypted crypto.EncryptedData
	if err := json.Unmarshal(data, &encrypted); err != nil {
		return "", err
	}

	return crypto.Decrypt(&encrypted, password)
}
