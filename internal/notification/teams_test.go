package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/0xJacky/pofile"
)

func TestTeamsNotifierSendsWorkflowWebhookPayload(t *testing.T) {
	withTranslationDict(t, map[string]pofile.Dict{
		"en": {},
	})

	var tokenForm map[string]string
	var tokenPath string
	var authHeader string
	var sentMessage map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token"):
			tokenPath = r.URL.Path
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			tokenForm = map[string]string{
				"client_id":     r.PostForm.Get("client_id"),
				"client_secret": r.PostForm.Get("client_secret"),
				"scope":         r.PostForm.Get("scope"),
				"grant_type":    r.PostForm.Get("grant_type"),
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-access-token"}`))
		case strings.HasPrefix(r.URL.Path, "/powerautomate/"):
			authHeader = r.Header.Get("Authorization")
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if err := json.Unmarshal(payload, &sentMessage); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalTokenURLTemplate := teamsTokenURLTemplate
	teamsTokenURLTemplate = server.URL + "/%s/oauth2/v2.0/token"
	t.Cleanup(func() {
		teamsTokenURLTemplate = originalTokenURLTemplate
	})

	message := &ExternalMessage{Notification: &model.Notification{
		Title:   "MES Exception Alert",
		Content: "Line1\nLine2",
	}}

	err := message.SendWithConfigContext(context.Background(), "teams", "en", map[string]string{
		"tenant_id":            "0ae51e19-07c8-4e4b-bb6d-648ee58410f4",
		"client_id":            "client-1",
		"client_secret":        "secret-1",
		"workflow_webhook_url": server.URL + "/powerautomate/automations/direct/workflows/9985c2d3ed3b4a84bdbe8dd1c0d36e7d/triggers/manual/paths/invoke?api-version=1",
		"message_body":         `[{"type":"TextBlock","text":"{{title}}","weight":"Bolder","size":"Large","color":"red"},{"type":"FactSet","facts":[{"title":"Content","value":"{{content}}"}]}]`,
	})
	if err != nil {
		t.Fatalf("SendWithConfigContext() error = %v", err)
	}

	if tokenPath != "/0ae51e19-07c8-4e4b-bb6d-648ee58410f4/oauth2/v2.0/token" {
		t.Fatalf("token path = %q", tokenPath)
	}
	if tokenForm["client_id"] != "client-1" {
		t.Fatalf("client_id = %q", tokenForm["client_id"])
	}
	if tokenForm["scope"] != "https://service.flow.microsoft.com//.default" {
		t.Fatalf("scope = %q", tokenForm["scope"])
	}
	if authHeader != "Bearer test-access-token" {
		t.Fatalf("Authorization = %q", authHeader)
	}
	if got, _ := sentMessage["type"].(string); got != "message" {
		t.Fatalf("type = %q", got)
	}
	attachments, ok := sentMessage["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v", sentMessage["attachments"])
	}
	attachment, ok := attachments[0].(map[string]any)
	if !ok {
		t.Fatalf("attachment = %#v", attachments[0])
	}
	if got, _ := attachment["contentType"].(string); got != "application/vnd.microsoft.card.adaptive" {
		t.Fatalf("contentType = %q", got)
	}
	contentObj, ok := attachment["content"].(map[string]any)
	if !ok {
		t.Fatalf("content = %#v", attachment["content"])
	}
	bodyBlocks, ok := contentObj["body"].([]any)
	if !ok || len(bodyBlocks) != 2 {
		t.Fatalf("body blocks = %#v", contentObj["body"])
	}
	firstBlock, _ := bodyBlocks[0].(map[string]any)
	if got, _ := firstBlock["text"].(string); got != "MES Exception Alert" {
		t.Fatalf("title text = %q", got)
	}
	if got, _ := firstBlock["color"].(string); got != "Attention" {
		t.Fatalf("normalized color = %q", got)
	}
	secondBlock, _ := bodyBlocks[1].(map[string]any)
	facts, _ := secondBlock["facts"].([]any)
	if len(facts) != 1 {
		t.Fatalf("facts = %#v", secondBlock["facts"])
	}
	fact, _ := facts[0].(map[string]any)
	if got, _ := fact["value"].(string); got != "Line1\nLine2" {
		t.Fatalf("fact content = %q", got)
	}
	if _, hasActions := contentObj["actions"]; hasActions {
		t.Fatalf("actions should not be present in fixed payload: %#v", contentObj["actions"])
	}
}

func TestBuildTeamsWorkflowPayloadFallbackTitle(t *testing.T) {
	payload, err := buildTeamsWorkflowPayload("", "content", "")
	if err != nil {
		t.Fatalf("buildTeamsWorkflowPayload() error = %v", err)
	}
	attachments, _ := payload["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("attachments len = %d", len(attachments))
	}
	attachment, _ := attachments[0].(map[string]any)
	contentObj, _ := attachment["content"].(map[string]any)
	bodyBlocks, _ := contentObj["body"].([]any)
	firstBlock, _ := bodyBlocks[0].(map[string]any)
	if got, _ := firstBlock["text"].(string); got != "Nginx UI Notification" {
		t.Fatalf("fallback title = %q", got)
	}
}

func TestBuildTeamsWorkflowPayloadRejectsInvalidMessageBody(t *testing.T) {
	_, err := buildTeamsWorkflowPayload("title", "content", "{not-json}")
	if err == nil {
		t.Fatal("expected invalid config error")
	}
}

func TestBuildTeamsWorkflowPayloadForcesMessageType(t *testing.T) {
	payload, err := buildTeamsWorkflowPayload("title", "content", `[{"type":"TextBlock","text":"{{title}}"}]`)
	if err != nil {
		t.Fatalf("buildTeamsWorkflowPayload() error = %v", err)
	}
	if payloadType, _ := payload["type"].(string); payloadType != "message" {
		t.Fatalf("payload type = %q", payloadType)
	}
}

func TestTeamsNotifierRejectsInvalidGlobalHTTPProxy(t *testing.T) {
	message := &ExternalMessage{Notification: &model.Notification{
		Title:   "Config Sync",
		Content: "ok",
	}}
	originalHTTPProxy := settings.HTTPSettings.HTTPProxy
	settings.HTTPSettings.HTTPProxy = "://bad-proxy"
	t.Cleanup(func() {
		settings.HTTPSettings.HTTPProxy = originalHTTPProxy
	})

	err := message.SendWithConfigContext(context.Background(), "teams", "en", map[string]string{
		"tenant_id":            "org-1",
		"client_id":            "client-1",
		"client_secret":        "secret-1",
		"workflow_webhook_url": "https://example.com/workflow",
	})
	if err == nil {
		t.Fatal("expected error for invalid global http proxy")
	}
}
