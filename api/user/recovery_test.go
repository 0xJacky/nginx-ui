package user

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRecoveryCodeUsesLowercaseBase36Format(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-z]{5}-[0-9a-z]{5}$`)

	for i := 0; i < 100; i++ {
		code, err := generateRecoveryCode()
		require.NoError(t, err)
		assert.Regexp(t, pattern, code)
	}
}

func TestGenerateRecoveryCodesReturnsRequestedCount(t *testing.T) {
	codes, err := generateRecoveryCodes(16)
	require.NoError(t, err)
	require.Len(t, codes, 16)

	pattern := regexp.MustCompile(`^[0-9a-z]{5}-[0-9a-z]{5}$`)
	for _, code := range codes {
		require.NotNil(t, code)
		assert.Regexp(t, pattern, code.Code)
	}
}
