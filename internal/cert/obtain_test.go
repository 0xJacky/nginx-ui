package cert

import (
	"testing"

	"github.com/go-acme/lego/v5/certcrypto"
)

func TestNewObtainRequestIncludesCommonNameOption(t *testing.T) {
	payload := &ConfigPayload{
		ServerName:       []string{"example.com", "www.example.com"},
		KeyType:          certcrypto.RSA2048,
		MustStaple:       true,
		EnableCommonName: true,
	}

	request := newObtainRequest(payload)

	if !request.EnableCommonName {
		t.Fatalf("EnableCommonName = false, want true")
	}
	if !request.MustStaple {
		t.Fatalf("MustStaple = false, want true")
	}
	if request.KeyType != certcrypto.RSA2048 {
		t.Fatalf("KeyType = %s, want %s", request.KeyType, certcrypto.RSA2048)
	}
	if len(request.Domains) != 2 || request.Domains[0] != "example.com" || request.Domains[1] != "www.example.com" {
		t.Fatalf("Domains = %#v, want example.com and www.example.com", request.Domains)
	}
}
