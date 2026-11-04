package settings

type OIDC struct {
	ClientId     string `json:"client_id" protected:"true"`
	ClientSecret string `json:"client_secret" protected:"true" sensitive:"true"`
	Endpoint     string `json:"endpoint" protected:"true"`
	RedirectUri  string `json:"redirect_uri" protected:"true"`
	Scopes       string `json:"scopes" protected:"true"`
	Identifier   string `json:"identifier" protected:"true"`
	// TrustUnverifiedEmail accepts the email claim as an account identifier
	// even when the provider does not assert email_verified=true. Only enable
	// it for providers that never emit email_verified but control the address.
	TrustUnverifiedEmail bool `json:"trust_unverified_email" protected:"true"`
}

var OIDCSettings = &OIDC{}
