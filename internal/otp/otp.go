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
	"strings"
	"time"
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

// TimeBar returns a visual progress bar for TOTP countdown
func TimeBar(remaining, period int) string {
	filled := int(float64(remaining) / float64(period) * 10)
	empty := 10 - filled
	return fmt.Sprintf("[%s%s] %ds",
		strings.Repeat("█", filled),
		strings.Repeat("░", empty),
		remaining)
}
