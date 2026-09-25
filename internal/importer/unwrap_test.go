package importer

import (
	"testing"

	"github.com/saadnvd1/xpass/internal/vault"
)

func TestFieldValueUnwrapsEveryType(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{map[string]interface{}{"string": ""}, ""},
		{map[string]interface{}{"creditCardNumber": "4111 1111"}, "4111 1111"},
		{map[string]interface{}{"concealed": "hunter2"}, "hunter2"},
		{map[string]interface{}{"email": map[string]interface{}{"email_address": "a@b.co", "provider": nil}}, "a@b.co"},
		{map[string]interface{}{"monthYear": float64(202912)}, "202912"},
		{map[string]interface{}{"address": map[string]interface{}{"street": "1 Main", "city": "Houston", "zip": ""}}, "Houston, 1 Main"},
		{"plain", "plain"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := fieldValue(c.in); got != c.want {
			t.Errorf("fieldValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMonthYear(t *testing.T) {
	if m, y, ok := monthYear(float64(202912)); !ok || m != "12" || y != "2029" {
		t.Errorf("202912 -> %q %q %v", m, y, ok)
	}
	if m, y, ok := monthYear("07/2031"); !ok || m != "07" || y != "2031" {
		t.Errorf("07/2031 -> %q %q %v", m, y, ok)
	}
}

func TestUnwrapRepairsWhatTheOldImporterStored(t *testing.T) {
	cases := map[string]string{
		"map[string:]":                                    "",
		"map[creditCardNumber:4111 1111 1111 1111]":       "4111 1111 1111 1111",
		"map[string:Note: keep this]":                     "Note: keep this",
		"map[email:map[email_address:a@b.co provider:]]":  "a@b.co",
		"map[creditCardType:visa]":                        "visa",
	}
	for in, want := range cases {
		if got, ok := Unwrap(in); !ok || got != want {
			t.Errorf("Unwrap(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, keep := range []string{"hunter2", "map[city:Houston state:TX street:1 Main]", "map[]", ""} {
		if got, ok := Unwrap(keep); ok || got != keep {
			t.Errorf("Unwrap(%q) changed it to %q", keep, got)
		}
	}
}

func TestRepairEntryFixesEveryStringField(t *testing.T) {
	e := vault.Entry{Name: "Sofi", Type: vault.TypeCreditCard, CardholderName: "map[string:]",
		CardNumber: "map[creditCardNumber:4111 1111]", CVV: "123",
		CustomFields: []vault.CustomField{{Name: "bank", Value: "map[string:SoFi]"}}}
	got := RepairEntry(&e)
	if e.CardholderName != "" || e.CardNumber != "4111 1111" || e.CVV != "123" || e.CustomFields[0].Value != "SoFi" {
		t.Fatalf("not repaired: %+v", e)
	}
	if len(got) != 3 {
		t.Errorf("changed = %v, want 3 fields", got)
	}
}
