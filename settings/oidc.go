package settings

type OIDC struct {
	ClientId     string `json:"client_id" protected:"true"`
	ClientSecret string `json:"client_secret" protected:"true"`
	Endpoint     string `json:"endpoint" protected:"true"`
	RedirectUri  string `json:"redirect_uri" protected:"true"`
	Scopes       string `json:"scopes" protected:"true"`
	Identifier   string `json:"identifier" protected:"true"`
}

var OIDCSettings = &OIDC{}
