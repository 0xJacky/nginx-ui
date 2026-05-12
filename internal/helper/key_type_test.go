package helper

import (
	"testing"

	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/stretchr/testify/assert"
)

func TestGetKeyTypeSupportsLegacyLegoV4Values(t *testing.T) {
	tests := []struct {
		name     string
		input    certcrypto.KeyType
		expected certcrypto.KeyType
	}{
		{name: "EC256", input: "P256", expected: certcrypto.EC256},
		{name: "EC384", input: "P384", expected: certcrypto.EC384},
		{name: "RSA2048", input: "2048", expected: certcrypto.RSA2048},
		{name: "RSA3072", input: "3072", expected: certcrypto.RSA3072},
		{name: "RSA4096", input: "4096", expected: certcrypto.RSA4096},
		{name: "RSA8192", input: "8192", expected: certcrypto.RSA8192},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetKeyType(tt.input))
			assert.True(t, IsValidKeyType(tt.input))
		})
	}
}

func TestGetKeyTypeAliasesIncludesLegacyAndCurrentValues(t *testing.T) {
	assert.ElementsMatch(t, []certcrypto.KeyType{certcrypto.EC256, "P256"}, GetKeyTypeAliases(certcrypto.EC256))
	assert.ElementsMatch(t, []certcrypto.KeyType{certcrypto.EC384, "P384"}, GetKeyTypeAliases("P384"))
	assert.ElementsMatch(t, []certcrypto.KeyType{certcrypto.RSA2048, "2048"}, GetKeyTypeAliases(certcrypto.RSA2048))
	assert.ElementsMatch(t, []certcrypto.KeyType{certcrypto.RSA3072, "3072"}, GetKeyTypeAliases("3072"))
	assert.ElementsMatch(t, []certcrypto.KeyType{certcrypto.RSA4096, "4096"}, GetKeyTypeAliases(certcrypto.RSA4096))
	assert.ElementsMatch(t, []certcrypto.KeyType{certcrypto.RSA8192, "8192"}, GetKeyTypeAliases("8192"))
}
