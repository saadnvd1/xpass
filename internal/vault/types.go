package vault

import "time"

// EntryType represents the kind of credential stored
type EntryType string

const (
	TypeLogin        EntryType = "login"
	TypeCreditCard   EntryType = "credit_card"
	TypeIdentity     EntryType = "identity"
	TypeSecureNote   EntryType = "secure_note"
	TypeSSHKey       EntryType = "ssh_key"
	TypeAPIKey       EntryType = "api_credential"
	TypeDatabase     EntryType = "database"
	TypeServer       EntryType = "server"
	TypeCryptoWallet EntryType = "crypto_wallet"
)

// Entry is the base credential record
type Entry struct {
	ID          string    `json:"id"`
	Type        EntryType `json:"type"`
	Name        string    `json:"name"`
	Tags        []string  `json:"tags"`
	Favorite    bool      `json:"favorite"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   string    `json:"createdAt"`
	UpdatedAt   string    `json:"updatedAt"`
	Version     int       `json:"version"`
	LastAccessed string   `json:"lastAccessed,omitempty"`
	AccessCount  int      `json:"accessCount,omitempty"`

	// Login fields
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	URL      string `json:"url,omitempty"`
	TOTP     *TOTP  `json:"totp,omitempty"`

	// Credit card fields
	CardholderName string `json:"cardholderName,omitempty"`
	CardNumber     string `json:"cardNumber,omitempty"`
	ExpiryMonth    string `json:"expiryMonth,omitempty"`
	ExpiryYear     string `json:"expiryYear,omitempty"`
	CVV            string `json:"cvv,omitempty"`
	PIN            string `json:"pin,omitempty"`

	// Secure note
	Content string `json:"content,omitempty"`

	// SSH key
	PrivateKey string `json:"privateKey,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	KeyType    string `json:"keyType,omitempty"`

	// API credential
	APIKey    string `json:"apiKey,omitempty"`
	APISecret string `json:"apiSecret,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`

	// Database
	DBType           string `json:"dbType,omitempty"`
	Host             string `json:"host,omitempty"`
	Port             int    `json:"port,omitempty"`
	Database         string `json:"database,omitempty"`
	ConnectionString string `json:"connectionString,omitempty"`

	// Server
	Protocol string `json:"protocol,omitempty"`

	// Crypto wallet
	WalletAddress string `json:"walletAddress,omitempty"`
	SeedPhrase    string `json:"seedPhrase,omitempty"`
	Network       string `json:"network,omitempty"`

	// Recovery codes (raw file content, encrypted with vault)
	RecoveryCodes string `json:"recoveryCodes,omitempty"`

	// Custom fields
	CustomFields []CustomField `json:"customFields,omitempty"`
}

type TOTP struct {
	Secret    string `json:"secret"`
	Algorithm string `json:"algorithm,omitempty"`
	Digits    int    `json:"digits,omitempty"`
	Period    int    `json:"period,omitempty"`
}

type CustomField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// Config holds vault settings
type Config struct {
	Version           string          `json:"version"`
	CreatedAt         string          `json:"createdAt"`
	LastSync          string          `json:"lastSync,omitempty"`
	RemoteURL         string          `json:"remoteUrl,omitempty"`
	DefaultTimeout    int             `json:"defaultTimeout"`
	ClipboardClearTime int            `json:"clipboardClearTime"`
	PasswordGenerator  PasswordGenConfig `json:"passwordGenerator"`
}

type PasswordGenConfig struct {
	Length         int  `json:"length"`
	Uppercase      bool `json:"uppercase"`
	Lowercase      bool `json:"lowercase"`
	Numbers        bool `json:"numbers"`
	Symbols        bool `json:"symbols"`
	ExcludeSimilar bool `json:"excludeSimilar"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Version:            "1.0.0",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		DefaultTimeout:     30,
		ClipboardClearTime: 30,
		PasswordGenerator: PasswordGenConfig{
			Length:         20,
			Uppercase:      true,
			Lowercase:      true,
			Numbers:        true,
			Symbols:        true,
			ExcludeSimilar: true,
		},
	}
}

// DisplayName returns a human-readable type name
func (t EntryType) DisplayName() string {
	switch t {
	case TypeLogin:
		return "Login"
	case TypeCreditCard:
		return "Credit Card"
	case TypeIdentity:
		return "Identity"
	case TypeSecureNote:
		return "Secure Note"
	case TypeSSHKey:
		return "SSH Key"
	case TypeAPIKey:
		return "API Key"
	case TypeDatabase:
		return "Database"
	case TypeServer:
		return "Server"
	case TypeCryptoWallet:
		return "Crypto Wallet"
	default:
		return string(t)
	}
}

// Subtitle returns contextual info for list display
func (e *Entry) Subtitle() string {
	switch e.Type {
	case TypeLogin:
		if e.Username != "" {
			return e.Username
		}
		if e.Email != "" {
			return e.Email
		}
		return e.URL
	case TypeCreditCard:
		if len(e.CardNumber) >= 4 {
			return "****" + e.CardNumber[len(e.CardNumber)-4:]
		}
		return e.CardholderName
	case TypeSecureNote:
		if len(e.Content) > 40 {
			return e.Content[:40] + "..."
		}
		return e.Content
	case TypeSSHKey:
		return e.KeyType
	case TypeAPIKey:
		return e.Endpoint
	case TypeDatabase:
		return e.DBType + " @ " + e.Host
	case TypeServer:
		return e.Host
	default:
		return string(e.Type)
	}
}
