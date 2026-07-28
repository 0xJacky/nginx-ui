package nodeauth

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialEnvelopeAuthenticatesVersionPurposeAndInstance(t *testing.T) {
	originalSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = originalSecret
		settings.NodeSettings.InstanceID = originalInstanceID
	})
	settings.CryptoSettings.Secret = "credential-root"
	settings.NodeSettings.InstanceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

	envelope, err := EncryptPrivateCredential("purpose-a", []byte("private material"))
	require.NoError(t, err)
	assert.Equal(t, credentialEnvelopeVersion, envelope[0])

	plaintext, err := DecryptPrivateCredential("purpose-a", envelope)
	require.NoError(t, err)
	assert.Equal(t, "private material", string(plaintext))

	_, err = DecryptPrivateCredential("purpose-b", envelope)
	require.Error(t, err)

	tampered := append([]byte(nil), envelope...)
	tampered[len(tampered)-1] ^= 1
	_, err = DecryptPrivateCredential("purpose-a", tampered)
	require.Error(t, err)

	settings.NodeSettings.InstanceID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	_, err = DecryptPrivateCredential("purpose-a", envelope)
	require.Error(t, err)
}
