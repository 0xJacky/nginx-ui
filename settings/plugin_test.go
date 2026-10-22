package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTrustedPublisher(t *testing.T) {
	for entry, want := range map[string][2]string{
		"RWkey":                  {"RWkey", ""},
		"  RWkey  ":              {"RWkey", ""},
		"RWkey Example Labs":     {"RWkey", "Example Labs"},
		"RWkey\tExample  Labs  ": {"RWkey", "Example  Labs"},
		"":                       {"", ""},
		"untrusted comment: minisign public key 1234\nRWkey\n": {"RWkey", ""},
	} {
		key, name := ParseTrustedPublisher(entry)
		assert.Equal(t, want, [2]string{key, name}, entry)
	}
}

func TestTrustedKeysDropNamesAndBlankEntries(t *testing.T) {
	p := &Plugin{TrustedPublicKeys: []string{"RWa Example", "", "RWb"}}
	assert.Equal(t, []string{"RWa", "RWb"}, p.TrustedKeys())
}
