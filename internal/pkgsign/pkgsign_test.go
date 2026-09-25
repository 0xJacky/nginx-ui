package pkgsign

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKey(t *testing.T) (string, minisign.PrivateKey) {
	t.Helper()
	public, private, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encoded, err := public.MarshalText()
	require.NoError(t, err)
	return string(encoded), private
}

// prehashedSignature is what "minisign -S" writes by default.
func prehashedSignature(t *testing.T, message []byte, key minisign.PrivateKey) []byte {
	t.Helper()
	reader := minisign.NewReader(bytes.NewReader(message))
	_, err := io.Copy(io.Discard, reader)
	require.NoError(t, err)
	return reader.Sign(key)
}

func TestVerifyBytesAcceptsBothAlgorithms(t *testing.T) {
	public, private := newKey(t)
	message := []byte("abc  plugin.json\n")

	for name, signature := range map[string][]byte{
		"legacy":    minisign.Sign(private, message),
		"prehashed": prehashedSignature(t, message, private),
	} {
		t.Run(name, func(t *testing.T) {
			keyID, err := VerifyBytes(message, signature, []string{public})
			require.NoError(t, err)
			assert.Equal(t, private.ID(), keyID)

			_, err = VerifyBytes([]byte("tampered"), signature, []string{public})
			assert.ErrorIs(t, err, ErrSignatureInvalid)
		})
	}
}

func TestVerifyBytesReportsTheKeyProblems(t *testing.T) {
	public, private := newKey(t)
	other, _ := newKey(t)
	message := []byte("message")
	signature := minisign.Sign(private, message)

	keyID, err := VerifyBytes(message, signature, []string{other})
	assert.ErrorIs(t, err, ErrSignatureKeyUnknown)
	assert.Equal(t, private.ID(), keyID)

	_, err = VerifyBytes(message, signature, nil)
	assert.ErrorIs(t, err, ErrTrustedSignatureKeysEmpty)

	_, err = VerifyBytes(message, signature, []string{"not a key"})
	assert.ErrorIs(t, err, ErrTrustedSignatureKeysBad)

	_, err = VerifyBytes(message, []byte("not a signature"), []string{public})
	assert.ErrorIs(t, err, ErrSignatureInvalid)
}

func TestKeyID(t *testing.T) {
	_, private := newKey(t)
	keyID, err := KeyID(minisign.Sign(private, []byte("message")))
	require.NoError(t, err)
	assert.Equal(t, private.ID(), keyID)

	_, err = KeyID([]byte("garbage"))
	assert.ErrorIs(t, err, ErrSignatureInvalid)
}
