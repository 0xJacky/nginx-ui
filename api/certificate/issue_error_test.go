package certificate

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

var testCertErrScope = cosy.NewErrorScope("cert")

func TestIssueErrorResponseCarriesWrappedCosyError(t *testing.T) {
	cErr := testCertErrScope.NewWithParams(50058, "HTTP-01 challenge route check failed for {0}: {1}", "example.com", "unexpected status 404")
	err := fmt.Errorf("issue certificate: %w", cErr)
	hint := &acmehint.Hint{Code: acmehint.CodePort80Unreachable, Message: "m"}

	resp := issueErrorResponse(err, hint)

	assert.Equal(t, Error, resp.Status)
	assert.Equal(t, err.Error(), resp.Message)
	assert.Same(t, hint, resp.Hint)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "cert", resp.Error.Scope)
	assert.Equal(t, int32(50058), resp.Error.Code)
	assert.Equal(t, "HTTP-01 challenge route check failed for {0}: {1}", resp.Error.Message)
	assert.Equal(t, []string{"example.com", "unexpected status 404"}, resp.Error.Params)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"status": "error",
		"message": "issue certificate: HTTP-01 challenge route check failed for example.com: unexpected status 404",
		"hint": {"code": "port80_unreachable", "message": "m"},
		"error": {
			"scope": "cert",
			"code": 50058,
			"message": "HTTP-01 challenge route check failed for {0}: {1}",
			"params": ["example.com", "unexpected status 404"]
		}
	}`, string(raw))
}

func TestIssueErrorResponseOmitsErrorForPlainErrors(t *testing.T) {
	resp := issueErrorResponse(errors.New("boom"), nil)

	assert.Equal(t, Error, resp.Status)
	assert.Equal(t, "boom", resp.Message)
	assert.Nil(t, resp.Error)
	assert.Nil(t, resp.Hint)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"error","message":"boom"}`, string(raw))
}
