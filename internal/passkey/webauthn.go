package passkey

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var instance *webauthn.WebAuthn

func Init() {
	options := &settings.WebAuthnSettings

	if !Enabled() {
		logger.Debug("WebAuthn settings are not configured")
		return
	}
	requireResidentKey := true
	var err error
	instance, err = webauthn.New(&webauthn.Config{
		RPDisplayName: options.RPDisplayName,
		RPID:          options.RPID,
		RPOrigins:     options.RPOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: &requireResidentKey,
			UserVerification:   "required",
		},
	})

	if err != nil {
		logger.Fatal(err)
	}
}

func Enabled() bool {
	options := &settings.WebAuthnSettings
	if options.RPDisplayName == "" || options.RPID == "" || len(options.RPOrigins) == 0 {
		return false
	}
	return true
}

func GetInstance() *webauthn.WebAuthn {
	return instance
}
