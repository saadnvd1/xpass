package importer

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// fieldValue turns a 1Password field value into the text it holds. 1Password
// wraps each value in a one-key object named after its type — {"string": ""},
// {"creditCardNumber": "4111…"}, {"email": {...}}, {"monthYear": 202912} — and
// printing that with %v stored Go's map syntax ("map[string:]") in the vault.
func fieldValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case map[string]interface{}:
		for _, k := range []string{"concealed", "totp", "string", "creditCardNumber"} {
			if x, ok := t[k]; ok {
				return fieldValue(x)
			}
		}
		if len(t) == 1 {
			for _, x := range t {
				return fieldValue(x)
			}
		}
		// Several parts (an address, an email with its provider): the
		// non-empty ones, in a stable order.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			if s := fieldValue(t[k]); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case []interface{}:
		var parts []string
		for _, x := range t {
			if s := fieldValue(x); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprintf("%v", v)
}

// monthYear splits 1Password's expiry into month and year: the number YYYYMM
// (202912), or text "MM/YYYY".
func monthYear(v interface{}) (month, year string, ok bool) {
	s := fieldValue(v)
	if parts := strings.Split(s, "/"); len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
	}
	if len(s) == 6 {
		if _, err := strconv.Atoi(s); err == nil {
			return s[4:], s[:4], true
		}
	}
	return "", "", false
}

// 1Password's single-value field types: what the old importer stored as
// "map[<type>:<value>]". Anything else (an address has several keys) is left.
var single = map[string]bool{
	"string": true, "concealed": true, "totp": true, "creditCardNumber": true, "creditCardType": true,
	"phone": true, "url": true, "date": true, "monthYear": true, "menu": true, "gender": true, "reference": true,
}

var wrapped = regexp.MustCompile(`(?s)^map\[([A-Za-z]+):(.*)\]$`)
var wrappedEmail = regexp.MustCompile(`^map\[email:map\[email_address:(\S*)( provider:\S*)?\]\]$`)

// Unwrap repairs one stored value: "map[string:]" -> "", "map[creditCardNumber:4111 1111]"
// -> "4111 1111", an email's nested map -> the address. Anything else is
// returned unchanged, with false.
func Unwrap(s string) (string, bool) {
	if m := wrappedEmail.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	if m := wrapped.FindStringSubmatch(s); m != nil && single[m[1]] {
		return m[2], true
	}
	return s, false
}
