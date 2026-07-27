package cert

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certificate"
)

func TestNormalizeCertificateIdentifiers(t *testing.T) {
	info, err := normalizeCertificateIdentifiers([]string{
		" example.com ", "203.0.113.8", "[2001:db8::1]", "2001:db8::1",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"example.com", "203.0.113.8", "2001:db8::1"}
	if len(info.Values) != len(want) {
		t.Fatalf("Values = %#v, want %#v", info.Values, want)
	}
	for i := range want {
		if info.Values[i] != want[i] {
			t.Fatalf("Values[%d] = %q, want %q", i, info.Values[i], want[i])
		}
	}
	if !info.HasIP {
		t.Fatal("HasIP = false, want true")
	}
}

func TestNormalizeAndValidateIdentifiersRejectsUnsupportedIPCombinations(t *testing.T) {
	tests := []struct {
		name    string
		payload ConfigPayload
	}{
		{
			name: "placeholder identifier",
			payload: ConfigPayload{
				ServerName:      []string{"_"},
				ChallengeMethod: HTTP01,
			},
		},
		{
			name: "IP with DNS challenge",
			payload: ConfigPayload{
				ServerName:      []string{"203.0.113.8"},
				ChallengeMethod: DNS01,
			},
		},
		{
			name: "IP with wildcard",
			payload: ConfigPayload{
				ServerName:      []string{"203.0.113.8", "*.example.com"},
				ChallengeMethod: HTTP01,
			},
		},
		{
			name: "IPv6 address with zone",
			payload: ConfigPayload{
				ServerName:      []string{"fe80::1%en0"},
				ChallengeMethod: HTTP01,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NormalizeAndValidateIdentifiers(&tt.payload); err == nil {
				t.Fatal("NormalizeAndValidateIdentifiers() error = nil")
			}
		})
	}
}

func TestNormalizeAndValidateIdentifiersDefaultsToHTTP01(t *testing.T) {
	payload := &ConfigPayload{ServerName: []string{"203.0.113.8"}}
	if err := NormalizeAndValidateIdentifiers(payload); err != nil {
		t.Fatal(err)
	}
	if payload.ChallengeMethod != HTTP01 {
		t.Fatalf("ChallengeMethod = %q, want %q", payload.ChallengeMethod, HTTP01)
	}
}

func TestResolveCertificateProfile(t *testing.T) {
	tests := []struct {
		name       string
		payload    *ConfigPayload
		advertised map[string]string
		want       string
		wantErr    bool
	}{
		{
			name:       "IP selects short-lived profile",
			payload:    &ConfigPayload{ServerName: []string{"203.0.113.8"}},
			advertised: map[string]string{"classic": "Classic", "shortlived": "Short-lived"},
			want:       shortLivedCertificateProfile,
		},
		{
			name:       "domain keeps default profile",
			payload:    &ConfigPayload{ServerName: []string{"example.com"}},
			advertised: map[string]string{"classic": "Classic", "shortlived": "Short-lived"},
		},
		{
			name:       "explicit advertised profile is preserved",
			payload:    &ConfigPayload{ServerName: []string{"example.com"}, Profile: " tlsserver "},
			advertised: map[string]string{"tlsserver": "TLS server"},
			want:       "tlsserver",
		},
		{
			name:       "resource profile is preserved",
			payload:    &ConfigPayload{ServerName: []string{"203.0.113.8"}, Resource: &model.CertificateResource{Resource: &certificate.Resource{Profile: "shortlived"}}},
			advertised: map[string]string{"shortlived": "Short-lived"},
			want:       "shortlived",
		},
		{
			name:       "custom CA without profiles keeps empty profile",
			payload:    &ConfigPayload{ServerName: []string{"203.0.113.8"}},
			advertised: nil,
		},
		{
			name:       "IP fails when advertised profiles exclude short-lived",
			payload:    &ConfigPayload{ServerName: []string{"203.0.113.8"}},
			advertised: map[string]string{"classic": "Classic"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolveCertificateProfile(tt.payload, tt.advertised)
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolveCertificateProfile() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.payload.Profile != tt.want {
				t.Fatalf("Profile = %q, want %q", tt.payload.Profile, tt.want)
			}
		})
	}
}
