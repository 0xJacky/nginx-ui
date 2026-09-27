package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pebbleWrongServerErr = `obtain cert error: resolver: one or more domains had a problem: [bad.e2e.test: invalid authorization: acme: error: 403 :: urn:ietf:params:acme:error:unauthorized :: Non-200 status code from HTTP: http://bad.e2e.test:80/.well-known/acme-challenge/GLVV returned 404]`

func stubDiagnoseForHint(t *testing.T, fn func(ctx context.Context, domains []string, opts acmehint.Options) []acmehint.Diagnostic) {
	t.Helper()
	orig := diagnoseForHint
	diagnoseForHint = fn
	t.Cleanup(func() { diagnoseForHint = orig })
}

func TestIssueFailureHintUsesDNSEvidenceForHTTP01(t *testing.T) {
	var gotDomains []string
	stubDiagnoseForHint(t, func(ctx context.Context, domains []string, _ acmehint.Options) []acmehint.Diagnostic {
		_, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline, "diagnostics must be bounded by a timeout")
		gotDomains = domains
		return []acmehint.Diagnostic{{
			Level: acmehint.LevelWarning,
			Code:  acmehint.CodeDNSPointsElsewhere,
			Params: map[string]string{
				"domain":   "bad.e2e.test",
				"resolved": "10.30.0.20",
				"local":    "10.30.0.10",
			},
		}}
	})

	payload := &cert.ConfigPayload{ServerName: []string{"bad.e2e.test"}, ChallengeMethod: cert.HTTP01}
	hint := issueFailureHint(context.Background(), payload, errors.New(pebbleWrongServerErr))

	require.NotNil(t, hint)
	assert.Equal(t, []string{"bad.e2e.test"}, gotDomains)
	assert.Equal(t, acmehint.CodeDNSPointsElsewhere, hint.Code)
	assert.Equal(t, "10.30.0.20", hint.Params["elsewhere"])
	assert.NotEmpty(t, hint.Message)
}

func TestIssueFailureHintSkipsDiagnosticsForDNS01(t *testing.T) {
	stubDiagnoseForHint(t, func(context.Context, []string, acmehint.Options) []acmehint.Diagnostic {
		t.Fatal("DNS-01 failures must not trigger address diagnostics")
		return nil
	})

	payload := &cert.ConfigPayload{ServerName: []string{"example.com"}, ChallengeMethod: "dns01"}
	hint := issueFailureHint(context.Background(), payload, errors.New("obtain cert error: acme: error: 429 :: urn:ietf:params:acme:error:rateLimited :: too many certificates"))

	require.NotNil(t, hint)
	assert.Equal(t, acmehint.CodeRateLimited, hint.Code)
}

func TestIssueFailureHintNilError(t *testing.T) {
	assert.Nil(t, issueFailureHint(context.Background(), &cert.ConfigPayload{}, nil))
}

func TestIssueCertResponseHintJSON(t *testing.T) {
	raw, err := json.Marshal(IssueCertResponse{Status: Error, Message: "boom"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"hint"`)

	raw, err = json.Marshal(IssueCertResponse{
		Status:  Error,
		Message: "boom",
		Hint:    &acmehint.Hint{Code: acmehint.CodePort80Unreachable, Message: "m", Params: map[string]string{"domain": "a"}},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"error","message":"boom","hint":{"code":"port80_unreachable","message":"m","params":{"domain":"a"}}}`, string(raw))
}
