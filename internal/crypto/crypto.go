package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	KeyLength        = 32     // 256 bits
	SaltLength       = 32     // 256 bits
	IVLength         = 16     // 128 bits (GCM nonce is 12, but we use 16 for compat)
	NonceLength      = 12     // GCM standard nonce
	AuthTagLength    = 16     // 128 bits
	PBKDF2Iterations = 600000 // OWASP recommended minimum
)

// EncryptedData matches the xpass-cli vault format for compatibility
type EncryptedData struct {
	Salt    string `json:"salt"`
	IV      string `json:"iv"`
	Data    string `json:"data"`
	AuthTag string `json:"authTag"`
	Version string `json:"version"`
}

// DeriveKey derives an encryption key from password + salt using PBKDF2-SHA256
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeyLength, sha256.New)
}

// GenerateSalt returns a cryptographically random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLength)
	_, err := rand.Read(salt)
	return salt, err
}

// Encrypt encrypts plaintext using AES-256-GCM, compatible with xpass-cli format
func Encrypt(plaintext string, password string) (*EncryptedData, error) {
	salt, err := GenerateSalt()
	if err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	key := DeriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	// Use 12-byte nonce for GCM (standard)
	nonce := make([]byte, NonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	// GCM Seal appends the auth tag to the ciphertext
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Split ciphertext and auth tag (last 16 bytes)
	ciphertext := sealed[:len(sealed)-AuthTagLength]
	authTag := sealed[len(sealed)-AuthTagLength:]

	// Store IV as 16 bytes (pad nonce with zeros) for xpass-cli compat
	iv := make([]byte, IVLength)
	copy(iv, nonce)

	return &EncryptedData{
		Salt:    hex.EncodeToString(salt),
		IV:      hex.EncodeToString(iv),
		Data:    hex.EncodeToString(ciphertext),
		AuthTag: hex.EncodeToString(authTag),
		Version: "1.0",
	}, nil
}

// Decrypt decrypts data encrypted by Encrypt or xpass-cli
func Decrypt(data *EncryptedData, password string) (string, error) {
	salt, err := hex.DecodeString(data.Salt)
	if err != nil {
		return "", fmt.Errorf("decoding salt: %w", err)
	}

	iv, err := hex.DecodeString(data.IV)
	if err != nil {
		return "", fmt.Errorf("decoding IV: %w", err)
	}

	ciphertext, err := hex.DecodeString(data.Data)
	if err != nil {
		return "", fmt.Errorf("decoding data: %w", err)
	}

	authTag, err := hex.DecodeString(data.AuthTag)
	if err != nil {
		return "", fmt.Errorf("decoding auth tag: %w", err)
	}

	key := DeriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	// Use first 12 bytes of IV as nonce (GCM standard)
	nonce := iv[:NonceLength]

	// Reconstruct sealed data (ciphertext + auth tag)
	sealed := append(ciphertext, authTag...)

	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", errors.New("decryption failed: invalid password or corrupted data")
	}

	return string(plaintext), nil
}

// HashPassword creates a SHA-256 hash (for session verification only)
func HashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

// SecureCompare does constant-time string comparison
func SecureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal([]byte(a), []byte(b))
}

// GeneratePassword creates a cryptographically random password
func GeneratePassword(length int, upper, lower, numbers, symbols bool) (string, error) {
	if length <= 0 {
		length = 20
	}

	upperChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars := "abcdefghijklmnopqrstuvwxyz"
	numberChars := "0123456789"
	symbolChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"

	var charset string
	if upper {
		charset += upperChars
	}
	if lower {
		charset += lowerChars
	}
	if numbers {
		charset += numberChars
	}
	if symbols {
		charset += symbolChars
	}
	if charset == "" {
		charset = lowerChars + numberChars
	}

	password := make([]byte, length)
	max := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}

	// Ensure at least one char from each selected set
	required := []string{}
	if upper {
		required = append(required, upperChars)
	}
	if lower {
		required = append(required, lowerChars)
	}
	if numbers {
		required = append(required, numberChars)
	}
	if symbols {
		required = append(required, symbolChars)
	}

	for i, chars := range required {
		if i >= length {
			break
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		pos, err := rand.Int(rand.Reader, big.NewInt(int64(length)))
		if err != nil {
			return "", err
		}
		password[pos.Int64()] = chars[n.Int64()]
	}

	return string(password), nil
}

// GenerateID creates a random hex ID
func GenerateID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GeneratePassphrase creates a random passphrase from a word list
func GeneratePassphrase(wordCount int, separator string) (string, error) {
	if wordCount <= 0 {
		wordCount = 4
	}
	if separator == "" {
		separator = "-"
	}

	words := []string{
		"acid", "acorn", "acre", "acts", "afar", "afoot", "aged", "agent", "agile", "aging",
		"agree", "ahead", "aide", "alarm", "album", "alert", "alibi", "alien", "alike", "alive",
		"alley", "allot", "allow", "alloy", "alone", "alpha", "altar", "alter", "amend", "ample",
		"amuse", "angel", "anger", "angle", "angry", "ankle", "apple", "apply", "apron", "arena",
		"argue", "arise", "armor", "army", "aroma", "arrow", "atlas", "atom", "attic", "audio",
		"audit", "avoid", "await", "awake", "award", "aware", "bacon", "badge", "badly", "bagel",
		"baker", "balmy", "banjo", "barge", "baron", "basic", "basin", "batch", "blade", "blame",
		"blank", "blast", "blaze", "bleak", "blend", "bless", "blind", "bliss", "block", "bloom",
		"blown", "bluff", "blunt", "blush", "board", "bogus", "bolt", "bonus", "boost", "booth",
		"brake", "brand", "brass", "brave", "bread", "breed", "brick", "bride", "brief", "bring",
		"brisk", "broad", "broke", "brook", "broom", "broth", "brown", "brush", "brute", "buddy",
		"budge", "build", "bunch", "cabin", "cable", "cache", "cadet", "camel", "candy", "canoe",
		"cargo", "carry", "carve", "catch", "cause", "cedar", "chain", "chair", "chalk", "champ",
		"chaos", "charm", "chart", "chase", "cheap", "check", "cheek", "cheer", "chess", "chest",
		"chill", "chirp", "chord", "chunk", "cider", "cigar", "cinch", "civic", "claim", "clamp",
		"clash", "clasp", "class", "clean", "clear", "clerk", "click", "cliff", "climb", "cling",
		"cloak", "clock", "clone", "close", "cloth", "cloud", "clown", "coach", "coast", "cobra",
		"coral", "craft", "crane", "crash", "crate", "crave", "crawl", "crazy", "cream", "creek",
		"crest", "crime", "crisp", "cross", "crowd", "crown", "crude", "crush", "crust", "curve",
		"cycle", "dairy", "dance", "delta", "dense", "depot", "depth", "digit", "diver", "dizzy",
		"dodge", "donor", "doubt", "draft", "drain", "drama", "dream", "drift", "drill", "drive",
		"drone", "drown", "dusk", "eagle", "earth", "eight", "elbow", "elite", "ember", "empty",
		"enemy", "enjoy", "entry", "equal", "erase", "essay", "evade", "event", "every", "exact",
		"exile", "fable", "faith", "feast", "fence", "ferry", "fiber", "field", "fifty", "fight",
		"flame", "flash", "flask", "fleet", "flesh", "float", "flock", "flood", "floor", "flour",
		"flown", "fluid", "flush", "focal", "focus", "force", "forge", "forum", "found", "frail",
		"frame", "frank", "fraud", "fresh", "frost", "fruit", "gauge", "ghost", "giant", "gland",
		"glare", "glass", "gleam", "glide", "globe", "gloom", "glory", "glove", "grace", "grain",
		"grand", "grape", "grasp", "grave", "greed", "grill", "grind", "grove", "guard", "guide",
		"habit", "harsh", "haven", "heart", "hedge", "honor", "house", "human", "humor", "ivory",
		"jewel", "juice", "knock", "label", "lance", "laser", "latch", "lemon", "level", "light",
		"lilac", "linen", "logic", "lunar", "mango", "manor", "maple", "march", "marsh", "merit",
		"metal", "might", "minor", "model", "moose", "mound", "nerve", "noble", "north", "novel",
		"ocean", "orbit", "order", "other", "outer", "oxide", "ozone", "paint", "panel", "patch",
		"pearl", "phase", "piano", "pilot", "pixel", "plank", "plant", "plaza", "plumb", "plume",
		"polar", "pouch", "power", "press", "prism", "prize", "proof", "proud", "pulse", "quake",
		"quest", "quick", "radar", "raise", "ranch", "rapid", "raven", "realm", "reign", "rider",
		"ridge", "rival", "river", "roast", "robin", "robot", "rover", "royal", "saint", "salad",
		"scale", "scope", "scout", "shape", "shark", "shelf", "shell", "shift", "shine", "shore",
		"shrub", "siege", "silk", "skill", "skull", "slate", "slice", "slope", "smart", "smith",
		"smoke", "snail", "solar", "solid", "south", "spark", "spear", "spice", "spoke", "staff",
		"stage", "stake", "stamp", "stand", "stark", "steam", "steel", "steep", "stern", "stone",
		"storm", "story", "stove", "straw", "surge", "swamp", "swarm", "sword", "table", "tempo",
		"thorn", "tiger", "toast", "token", "torch", "tower", "trace", "track", "trade", "trait",
		"trend", "trial", "tribe", "trick", "trout", "trunk", "trust", "tulip", "ultra", "unity",
		"upper", "urban", "usage", "utile", "valid", "valor", "vapor", "vault", "verse", "vigor",
		"viola", "vivid", "voice", "wagon", "watch", "water", "whale", "wheat", "wheel", "width",
		"world", "yacht", "yield", "youth", "zebra",
	}

	selected := make([]string, wordCount)
	max := big.NewInt(int64(len(words)))
	for i := 0; i < wordCount; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		selected[i] = words[n.Int64()]
	}

	return strings.Join(selected, separator), nil
}
