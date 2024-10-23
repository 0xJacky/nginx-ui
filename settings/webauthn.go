package settings

type WebAuthn struct {
	RPDisplayName string   `json:"rp_display_name"`
	RPID          string   `json:"rpid"`
	RPOrigins     []string `json:"rp_origins"`
}

var WebAuthnSettings = &WebAuthn{}
