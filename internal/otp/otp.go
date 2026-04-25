package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"hash"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/saadnvd1/xpass/internal/vault"
)

// Generate returns the current TOTP code, seconds remaining, and period
func Generate(secret string, algorithm string, digits int, period int) (string, int, int) {
	if algorithm == "" {
		algorithm = "SHA1"
	}
	if digits == 0 {
		digits = 6
	}
	if period == 0 {
		period = 30
	}

	now := time.Now().Unix()
	counter := uint64(now) / uint64(period)
	remaining := period - int(now%int64(period))

	code := generateHOTP(secret, counter, algorithm, digits)
	return code, remaining, period
}

func generateHOTP(secret string, counter uint64, algorithm string, digits int) string {
	key := base32Decode(strings.ToUpper(secret))

	// Counter to bytes (big-endian)
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)

	// HMAC
	var h func() hash.Hash
	switch strings.ToUpper(algorithm) {
	case "SHA256":
		h = sha256.New
	case "SHA512":
		h = sha512.New
	default:
		h = sha1.New
	}

	mac := hmac.New(h, key)
	mac.Write(msg)
	sum := mac.Sum(nil)

	// Dynamic truncation
	offset := sum[len(sum)-1] & 0x0f
	binCode := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]&0xff) << 16) |
		(uint32(sum[offset+2]&0xff) << 8) |
		uint32(sum[offset+3]&0xff)

	otp := binCode % uint32(math.Pow10(digits))
	return fmt.Sprintf("%0*d", digits, otp)
}

func base32Decode(encoded string) []byte {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	// Strip padding and non-base32 chars
	cleaned := strings.Map(func(r rune) rune {
		if strings.ContainsRune(alphabet, r) {
			return r
		}
		return -1
	}, strings.ToUpper(encoded))

	var bits string
	for _, c := range cleaned {
		idx := strings.IndexRune(alphabet, c)
		if idx >= 0 {
			bits += fmt.Sprintf("%05b", idx)
		}
	}

	var bytes []byte
	for i := 0; i+8 <= len(bits); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			if bits[i+j] == '1' {
				b |= 1 << uint(7-j)
			}
		}
		bytes = append(bytes, b)
	}

	return bytes
}

// ParseTOTPUri parses an otpauth:// URI into a TOTP struct
// Format: otpauth://totp/Label?secret=XXX&algorithm=SHA1&digits=6&period=30
func ParseTOTPUri(uri string) *vault.TOTP {
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

// ParseTOTPLabel extracts issuer and account from an otpauth:// URI
func ParseTOTPLabel(uri string) (issuer, account string) {
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		return "", ""
	}

	// Extract label between "otpauth://totp/" and "?"
	label := strings.TrimPrefix(uri, "otpauth://totp/")
	if idx := strings.Index(label, "?"); idx >= 0 {
		label = label[:idx]
	}
	label, _ = url.PathUnescape(label)

	// Check for issuer in query params
	if idx := strings.Index(uri, "?"); idx >= 0 {
		params := uri[idx+1:]
		for _, param := range strings.Split(params, "&") {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 && strings.ToLower(kv[0]) == "issuer" {
				issuer, _ = url.QueryUnescape(kv[1])
			}
		}
	}

	// Label format: "Issuer:Account" or just "Account"
	if strings.Contains(label, ":") {
		parts := strings.SplitN(label, ":", 2)
		if issuer == "" {
			issuer = strings.TrimSpace(parts[0])
		}
		account = strings.TrimSpace(parts[1])
	} else {
		account = strings.TrimSpace(label)
	}

	return issuer, account
}

// TimeBar returns a visual progress bar for TOTP countdown
func TimeBar(remaining, period int) string {
	filled := int(float64(remaining) / float64(period) * 10)
	empty := 10 - filled
	return fmt.Sprintf("[%s%s] %ds",
		strings.Repeat("█", filled),
		strings.Repeat("░", empty),
		remaining)
}
