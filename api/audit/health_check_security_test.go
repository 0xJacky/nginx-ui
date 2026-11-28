package audit

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	healthCheckProbeFormSecret = "synthetic-probe-form-value"
	healthCheckHeaderSecret    = "Bearer synthetic-probe-header-value"
	healthCheckBodySecret      = "synthetic-probe-body-value"
	healthCheckURLSecret       = "synthetic-probe-url-value"
)

// healthCheckConfigJSON is a health check configuration as the web UI sends
// it. None of its secrets sit under a credential-like field name, so only the
// route rule keeps them out of the audit log.
const healthCheckConfigJSON = `{"protocol":"https","method":"POST","path":"/health",` +
	`"headers":{"Authorization":"` + healthCheckHeaderSecret + `"},` +
	`"body":"{\"probe\":\"` + healthCheckBodySecret + `\"}",` +
	`"target_url":"https://probe:` + healthCheckURLSecret + `@example.com/health"}`

const healthCheckRequestJSON = `{"health_check_enabled":true,"check_interval":300,"timeout":10,` +
	`"health_check_config":` + healthCheckConfigJSON + `,` +
	`"probe_kind":"plugin:example","probe_config":{"login":"` + healthCheckProbeFormSecret + `"}}`

var healthCheckSecrets = []string{
	healthCheckProbeFormSecret,
	healthCheckHeaderSecret,
	healthCheckBodySecret,
	healthCheckURLSecret,
}

func assertNoHealthCheckSecret(t *testing.T, value string) {
	t.Helper()
	for _, secret := range healthCheckSecrets {
		assert.NotContains(t, value, secret)
	}
}

func TestSanitizeAuditLogRedactsHealthCheckRequests(t *testing.T) {
	cases := []auditCase{
		{
			method:  http.MethodPost,
			route:   "/api/site_navigation/health_check/:id",
			target:  "/api/site_navigation/health_check/7",
			reqBody: healthCheckRequestJSON,
		},
		{
			method: http.MethodPut,
			route:  "/api/site_navigation/health_check/sync",
			target: "/api/site_navigation/health_check/sync",
			reqBody: `{"site_key":"example.com:443","site_name":"example","host":"example.com:443",` +
				`"config":` + healthCheckRequestJSON + `}`,
		},
		{
			method: http.MethodPost,
			route:  "/api/site_navigation/test_health_check/:id",
			target: "/api/site_navigation/test_health_check/7",
			reqBody: `{"config":` + healthCheckConfigJSON + `,` +
				`"probe_kind":"plugin:example","probe_config":{"login":"` + healthCheckProbeFormSecret + `"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.route, func(t *testing.T) {
			tc.sessionLogs = `[{"sql":"UPDATE site_configs SET health_check_config = '` + healthCheckConfigJSON + `'"}]`
			tc.respBody = `{"message":"ok"}`
			tc.reqHeaders = map[string]string{"X-Request-Origin": "kept-header-value"}
			logMap := runAudit(t, tc)

			assert.Equal(t, redactedRequestBody, logMap["req_body"])
			assert.Equal(t, redactedSessionLogs, logMap["session_logs"])
			assertNoHealthCheckSecret(t, logMap["req_body"])
			assertNoHealthCheckSecret(t, logMap["session_logs"])

			// The entry still says which site was changed, how, and answered what.
			assert.Contains(t, logMap["req_url"], tc.target)
			assert.Contains(t, logMap["req_header"], "kept-header-value")
			assert.Equal(t, `{"message":"ok"}`, logMap["resp_body"])
		})
	}
}

func TestSanitizeAuditLogRedactsHealthCheckResponses(t *testing.T) {
	cases := []auditCase{
		{
			method: http.MethodGet,
			route:  "/api/site_navigation/health_check/:id",
			target: "/api/site_navigation/health_check/7",
			respBody: `{"id":7,"site_key":"example.com:443","probe_kind":"plugin:example",` +
				`"probe_config":{"login":"` + healthCheckProbeFormSecret + `"},` +
				`"health_check_config":` + healthCheckConfigJSON + `}`,
		},
		{
			method:   http.MethodGet,
			route:    "/api/site_navigation",
			target:   "/api/site_navigation",
			respBody: `{"data":[{"id":7,"name":"example.com","health_check_config":` + healthCheckConfigJSON + `}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.route, func(t *testing.T) {
			logMap := runAudit(t, tc)

			assert.Equal(t, redactedResponseBody, logMap["resp_body"])
			assertNoHealthCheckSecret(t, logMap["resp_body"])
			assert.Contains(t, logMap["req_url"], tc.target)
		})
	}
}

// TestSanitizeAuditLogKeepsOtherSiteNavigationBodies makes sure the health
// check rules do not hide the site navigation requests that carry no probe
// settings.
func TestSanitizeAuditLogKeepsOtherSiteNavigationBodies(t *testing.T) {
	body := `{"ordered_ids":[3,1,2]}`
	logMap := runAudit(t, auditCase{
		method:   http.MethodPost,
		route:    "/api/site_navigation/order",
		target:   "/api/site_navigation/order",
		reqBody:  body,
		respBody: `{"message":"Order updated successfully"}`,
	})
	assert.Equal(t, body, logMap["req_body"])
	assert.Equal(t, `{"message":"Order updated successfully"}`, logMap["resp_body"])

	status := `{"running":true,"enabled":true,"concurrency":5,"interval_seconds":300}`
	logMap = runAudit(t, auditCase{
		method:   http.MethodGet,
		route:    "/api/site_navigation/status",
		target:   "/api/site_navigation/status",
		respBody: status,
	})
	assert.Equal(t, status, logMap["resp_body"])
}
