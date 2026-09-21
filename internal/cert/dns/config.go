// Package dns is the registry of DNS providers the certificate pages offer.
//
// The core ships no vendor code. Provider schemas and the DNS-01 challenge
// implementation behind them come from registered sources: a builtin source
// describing the vendors the DNS record management supports, and one source
// per plugin that implements the dns01 capability.
package dns

// Configuration is the credential form of one provider. Keys are the
// environment variable names lego reads, values are what the user filled in.
type Configuration struct {
	Credentials map[string]string `json:"credentials"`
	Additional  map[string]string `json:"additional"`
}

// Links points at the vendor documentation.
type Links struct {
	API      string `json:"api"`
	GoClient string `json:"go_client"`
}

// Config is the value a DNS credential row stores. It is persisted as
// AES encrypted JSON and read back by the record management, so the field
// names and the JSON tags must stay stable.
type Config struct {
	Name          string         `json:"name"`
	Code          string         `json:"code"`
	Configuration *Configuration `json:"configuration,omitempty"`
	Links         *Links         `json:"links,omitempty"`
}
