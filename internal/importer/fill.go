package importer

import (
	"reflect"

	"github.com/saadnvd1/xpass/internal/vault"
)

// FillEmpty copies into `have` every field that is empty there and filled in
// `from` (a fresh parse of the same 1Password item). Nothing already filled is
// touched, so edits made since the import stand. Returns the JSON names of the
// fields it filled; never the values.
func FillEmpty(have *vault.Entry, from vault.Entry) []string {
	var filled []string
	hv, fv := reflect.ValueOf(have).Elem(), reflect.ValueOf(from)
	t := hv.Type()
	for i := 0; i < t.NumField(); i++ {
		h, f := hv.Field(i), fv.Field(i)
		if h.Kind() == reflect.String && h.CanSet() && h.String() == "" && f.String() != "" {
			h.SetString(f.String())
			filled = append(filled, jsonName(t.Field(i)))
		}
	}
	if (have.TOTP == nil || have.TOTP.Secret == "") && from.TOTP != nil && from.TOTP.Secret != "" {
		t := *from.TOTP
		have.TOTP = &t
		filled = append(filled, "totp")
	}
	return filled
}

// Match pairs each imported entry with the vault entry it became: same name,
// type and creation time (the import keeps 1Password's), else same name and
// type when that is unique. Unmatched imports are not added.
func Match(vaultEntries []vault.Entry, imported []vault.Entry) map[int]int {
	out := map[int]int{}
	for j, im := range imported {
		exact, loose := -1, []int{}
		for i, e := range vaultEntries {
			if e.Name != im.Name || e.Type != im.Type {
				continue
			}
			loose = append(loose, i)
			if e.CreatedAt == im.CreatedAt {
				exact = i
			}
		}
		if exact >= 0 {
			out[exact] = j
		} else if len(loose) == 1 {
			out[loose[0]] = j
		}
	}
	return out
}
