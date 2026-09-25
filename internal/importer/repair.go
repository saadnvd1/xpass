package importer

import (
	"reflect"

	"github.com/saadnvd1/xpass/internal/vault"
)

// RepairEntry unwraps every value the old 1Password importer stored as Go's
// print of a typed field ("map[string:]"), in place. It returns the JSON
// names of the fields it changed; never the values.
func RepairEntry(e *vault.Entry) []string {
	var changed []string
	fix := func(name string, s *string) {
		if v, ok := Unwrap(*s); ok {
			*s = v
			changed = append(changed, name)
		}
	}
	rv := reflect.ValueOf(e).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() == reflect.String && f.CanSet() {
			s := f.String()
			fix(jsonName(rt.Field(i)), &s)
			f.SetString(s)
		}
	}
	if e.TOTP != nil {
		fix("totp.secret", &e.TOTP.Secret)
	}
	for i := range e.CustomFields {
		fix("customFields."+e.CustomFields[i].Name, &e.CustomFields[i].Value)
	}
	return changed
}

func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	if tag == "" {
		return f.Name
	}
	return tag
}
