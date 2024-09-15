package settings

type WebAuthn struct {
	RPDisplayName string
	RPID          string
	RPOrigins     []string
}

var WebAuthnSettings = WebAuthn{}
