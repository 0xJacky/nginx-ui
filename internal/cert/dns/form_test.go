package dns

import (
	"testing"
)

func TestBuiltinProvidersCarryConsistentForms(t *testing.T) {
	for _, p := range (builtinSource{}).Providers() {
		t.Run(p.Code, func(t *testing.T) {
			if p.Form == nil || len(p.Form.Fields) == 0 {
				t.Fatal("builtin provider has no form")
			}
			if p.Links == nil || p.Links.API == "" {
				t.Fatal("builtin provider has no API docs link")
			}
			groups := map[string]string{}
			for _, f := range p.Form.Fields {
				if _, dup := groups[f.Key]; dup {
					t.Fatalf("field %s is declared twice", f.Key)
				}
				groups[f.Key] = f.Group
				if f.Key == "" || f.Label == "" {
					t.Fatalf("field %+v needs a key and a label", f)
				}
				if f.Group != FieldGroupCredential && f.Group != FieldGroupSetting {
					t.Fatalf("field %s has group %q", f.Key, f.Group)
				}
				if f.Unit != "" && f.Unit != FieldUnitSeconds {
					t.Fatalf("field %s has unit %q", f.Key, f.Unit)
				}
			}

			recommended := 0
			for _, m := range p.Form.Methods {
				if m.Recommended {
					recommended++
				}
				if len(m.Fields) == 0 {
					t.Fatalf("method %s lists no fields", m.Name)
				}
				for _, key := range m.Fields {
					if groups[key] != FieldGroupCredential {
						t.Fatalf("method %s uses %s which is not a credential field", m.Name, key)
					}
				}
			}
			if recommended > 1 {
				t.Fatalf("%d methods are recommended", recommended)
			}
			if len(p.Form.Methods) == 1 {
				t.Fatal("a single method should be left out")
			}
		})
	}
}

func TestBuiltinCloudflareForm(t *testing.T) {
	p, ok := GetProvider("cloudflare")
	if !ok || p.Form == nil {
		t.Fatal("cloudflare is missing")
	}

	byKey := map[string]FormField{}
	for _, f := range p.Form.Fields {
		byKey[f.Key] = f
	}
	if _, ok := byKey["CLOUDFLARE_API_KEY"]; ok {
		t.Fatal("aliases must not be in the form")
	}
	token := byKey["CF_DNS_API_TOKEN"]
	if !token.Secret || token.Optional || token.Label != "API token" || token.Group != FieldGroupCredential {
		t.Fatalf("CF_DNS_API_TOKEN = %+v", token)
	}
	if !byKey["CF_ZONE_API_TOKEN"].Optional {
		t.Fatal("CF_ZONE_API_TOKEN should be optional")
	}
	timeout := byKey["CLOUDFLARE_PROPAGATION_TIMEOUT"]
	if timeout.Default != "120" || timeout.Unit != FieldUnitSeconds || timeout.Group != FieldGroupSetting {
		t.Fatalf("CLOUDFLARE_PROPAGATION_TIMEOUT = %+v", timeout)
	}
	methods := p.Form.Methods
	if len(methods) != 2 || !methods[0].Recommended || methods[0].Name != "API token" {
		t.Fatalf("methods = %+v", methods)
	}
}

func TestProvidersListStripsForm(t *testing.T) {
	for _, p := range GetProvidersList() {
		if p.Form != nil {
			t.Fatalf("list entry %s carries a form", p.Code)
		}
	}
	p, ok := GetProvider("alidns")
	if !ok || p.Form == nil {
		t.Fatal("detail entry lost its form")
	}
}
