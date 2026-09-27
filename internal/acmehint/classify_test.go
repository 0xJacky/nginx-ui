package acmehint

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-acme/lego/v5/acme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real failures captured from the Pebble E2E run, as delivered to the UI.
const (
	pebbleWrongServer = `obtain cert error: resolver: one or more domains had a problem: [bad.e2e.test: invalid authorization: acme: error: 403 :: urn:ietf:params:acme:error:unauthorized :: Non-200 status code from HTTP: http://bad.e2e.test:80/.well-known/acme-challenge/GLVVabc returned 404]`
	pebbleUnreachable = `obtain cert error: resolver: one or more domains had a problem: [down.e2e.test: invalid authorization: acme: error: 400 :: urn:ietf:params:acme:error:connection :: Get "http://down.e2e.test:80/.well-known/acme-challenge/vyzabc": dial tcp 10.30.0.99:80: connect: no route to host]`
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		ev         Evidence
		wantCode   string
		wantParams map[string]string
	}{
		{
			name:       "pebble wrong server without evidence",
			err:        errors.New(pebbleWrongServer),
			wantCode:   CodeChallengeNotServed,
			wantParams: map[string]string{"domain": "bad.e2e.test"},
		},
		{
			name: "pebble wrong server resolving to a local address",
			err:  errors.New(pebbleWrongServer),
			ev: Evidence{
				Resolved: map[string][]string{"bad.e2e.test": {"10.30.0.10"}},
				Local:    []string{"127.0.0.1", "10.30.0.10"},
			},
			wantCode:   CodeChallengeNotServed,
			wantParams: map[string]string{"domain": "bad.e2e.test"},
		},
		{
			name: "pebble wrong server resolving elsewhere",
			err:  errors.New(pebbleWrongServer),
			ev: Evidence{
				Resolved: map[string][]string{"bad.e2e.test": {"10.30.0.20"}},
				Local:    []string{"10.30.0.10", "127.0.0.1"},
			},
			wantCode: CodeDNSPointsElsewhere,
			wantParams: map[string]string{
				"domain":    "bad.e2e.test",
				"resolved":  "10.30.0.20",
				"local":     "10.30.0.10",
				"elsewhere": "10.30.0.20",
			},
		},
		{
			name:     "pebble unreachable",
			err:      errors.New(pebbleUnreachable),
			wantCode: CodePort80Unreachable,
			wantParams: map[string]string{
				"domain":    "down.e2e.test",
				"remote_ip": "10.30.0.99",
				"port":      "80",
			},
		},
		{
			name:       "lets encrypt timeout",
			err:        errors.New(`obtain cert error: resolver: one or more domains had a problem: [example.com: invalid authorization: acme: error: 400 :: urn:ietf:params:acme:error:connection :: 203.0.113.7: Fetching http://example.com/.well-known/acme-challenge/abc: Timeout during connect (likely firewall problem)]`),
			wantCode:   CodePort80Unreachable,
			wantParams: map[string]string{"domain": "example.com", "remote_ip": "203.0.113.7"},
		},
		{
			name:       "lets encrypt nxdomain",
			err:        errors.New(`obtain cert error: resolver: one or more domains had a problem: [nope.example.com: invalid authorization: acme: error: 400 :: urn:ietf:params:acme:error:dns :: DNS problem: NXDOMAIN looking up A for nope.example.com - check that a DNS record exists for this domain]`),
			wantCode:   CodeDNSNotResolved,
			wantParams: map[string]string{"domain": "nope.example.com"},
		},
		{
			name:       "dns lookup failure without acme type",
			err:        errors.New(`Get "http://x.test/.well-known/acme-challenge/a": dial tcp: lookup x.test: no such host`),
			wantCode:   CodeDNSNotResolved,
			wantParams: nil,
		},
		{
			name:       "caa",
			err:        errors.New(`obtain cert error: resolver: one or more domains had a problem: [example.com: invalid authorization: acme: error: 403 :: urn:ietf:params:acme:error:caa :: CAA record for example.com prevents issuance]`),
			wantCode:   CodeCAAForbidden,
			wantParams: map[string]string{"domain": "example.com"},
		},
		{
			name:       "rate limited from string",
			err:        errors.New(`obtain cert error: acme: error: 429 :: POST :: https://acme-v02.api.letsencrypt.org/acme/new-order :: urn:ietf:params:acme:error:rateLimited :: too many certificates (5) already issued for this exact set of identifiers in the last 168h0m0s, retry after 2026-09-28 01:02:03 UTC: see https://letsencrypt.org/docs/rate-limits/#new-certificates-per-exact-set-of-hostnames`),
			wantCode:   CodeRateLimited,
			wantParams: map[string]string{"retry_after": "2026-09-28 01:02:03 UTC"},
		},
		{
			name:     "redirect to https that fails",
			err:      errors.New(`obtain cert error: resolver: one or more domains had a problem: [example.com: invalid authorization: acme: error: 403 :: urn:ietf:params:acme:error:unauthorized :: 203.0.113.7: Invalid response from https://example.com/.well-known/acme-challenge/abc: 404]`),
			wantCode: CodeChallengeRedirected,
			wantParams: map[string]string{
				"domain":    "example.com",
				"remote_ip": "203.0.113.7",
				"url":       "https://example.com/.well-known/acme-challenge/abc",
			},
		},
		{
			name:       "rejected identifier",
			err:        errors.New(`acme: error: 400 :: POST :: https://acme/new-order :: urn:ietf:params:acme:error:rejectedIdentifier :: Invalid identifiers requested :: Cannot issue for "foo.local": Domain name does not end with a valid public suffix (TLD)`),
			wantCode:   CodeIdentifierRejected,
			wantParams: nil,
		},
		{
			name:       "unrelated error",
			err:        errors.New("get acme user error: record not found"),
			wantCode:   CodeUnknown,
			wantParams: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.err, tt.ev)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, Message(tt.wantCode), got.Message)
			assert.NotEmpty(t, got.Message)
			assert.Equal(t, tt.wantParams, got.Params)
		})
	}
}

func TestClassifyNil(t *testing.T) {
	assert.Nil(t, Classify(nil, Evidence{}))
}

func TestClassifyMultipleDomainsPicksMostBlocking(t *testing.T) {
	msg := `obtain cert error: resolver: one or more domains had a problem: ` +
		`[a.example.com: invalid authorization: acme: error: 403 :: urn:ietf:params:acme:error:unauthorized :: Non-200 status code from HTTP: http://a.example.com:80/.well-known/acme-challenge/x returned 404] ` +
		`[b.example.com: invalid authorization: acme: error: 400 :: urn:ietf:params:acme:error:dns :: DNS problem: SERVFAIL looking up A for b.example.com]`

	got := Classify(errors.New(msg), Evidence{})
	require.NotNil(t, got)
	assert.Equal(t, CodeDNSNotResolved, got.Code)
	assert.Equal(t, "b.example.com", got.Params["domain"])
	assert.Equal(t, "a.example.com, b.example.com", got.Params["failed_domains"])
}

// multiError mimics lego's internal per-domain error map, which exposes its
// entries through Unwrap() []error.
type multiError struct {
	msg  string
	errs []error
}

func (m *multiError) Error() string   { return m.msg }
func (m *multiError) Unwrap() []error { return m.errs }

func TestClassifyStructured(t *testing.T) {
	t.Run("rate limited error carries retry-after", func(t *testing.T) {
		pd := &acme.ProblemDetails{
			Type:       acme.RateLimitedErrorType,
			Detail:     "too many new orders recently",
			HTTPStatus: 429,
		}
		// lego's sender returns the pointer form of RateLimitedError.
		var rateLimited error = &acme.RateLimitedError{ProblemDetails: pd, RetryAfter: 90 * time.Minute}
		err := fmt.Errorf("obtain: %w", rateLimited)

		got := Classify(err, Evidence{})
		require.NotNil(t, got)
		assert.Equal(t, CodeRateLimited, got.Code)
		assert.Equal(t, "1h30m0s", got.Params["retry_after"])
	})

	t.Run("per-domain problem details", func(t *testing.T) {
		pd := &acme.ProblemDetails{
			Type:       acme.UnauthorizedErrorType,
			Detail:     "Non-200 status code from HTTP: http://bad.e2e.test:80/.well-known/acme-challenge/x returned 404",
			HTTPStatus: 403,
		}
		inner := fmt.Errorf("invalid authorization: %w", pd)
		err := &multiError{
			msg:  "resolver: one or more domains had a problem: [bad.e2e.test: " + inner.Error() + "]",
			errs: []error{inner},
		}

		got := Classify(err, Evidence{
			Resolved: map[string][]string{"bad.e2e.test": {"2001:db8::2"}},
			Local:    []string{"2001:db8::1"},
		})
		require.NotNil(t, got)
		assert.Equal(t, CodeDNSPointsElsewhere, got.Code)
		assert.Equal(t, "bad.e2e.test", got.Params["domain"])
		assert.Equal(t, "2001:db8::2", got.Params["elsewhere"])
	})

	t.Run("compound problem uses sub-problem identifiers", func(t *testing.T) {
		pd := &acme.ProblemDetails{
			Type:   acme.CompoundErrorType,
			Detail: "Error creating new order",
			SubProblems: []acme.SubProblem{{
				Type:       acme.CaaErrorType,
				Detail:     "CAA record prevents issuance",
				Identifier: acme.Identifier{Type: "dns", Value: "c.example.com"},
			}},
		}
		got := Classify(pd, Evidence{})
		require.NotNil(t, got)
		assert.Equal(t, CodeCAAForbidden, got.Code)
		assert.Equal(t, "c.example.com", got.Params["domain"])
	})
}
