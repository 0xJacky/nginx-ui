package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/pofile"
)

func TestBuildMattermostWebhookURLAddsHooksPath(t *testing.T) {
	got := buildMattermostWebhookURL("https://mattermost.example.com", "test-webhook-token")
	want := "https://mattermost.example.com/hooks/test-webhook-token"
	if got != want {
		t.Fatalf("buildMattermostWebhookURL() = %q, want %q", got, want)
	}
}

func TestMattermostNotifierSendsWebhookPayload(t *testing.T) {
	withTranslationDict(t, map[string]pofile.Dict{
		"en": {},
	})

	var requestPath string
	var payload mattermostMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	msg := &ExternalMessage{
		Notification: &model.Notification{
			Title:   "External Notification Test",
			Content: "Mattermost content",
		},
	}

	err := msg.SendWithConfig("mattermost", "en", map[string]string{
		"url":      server.URL,
		"token":    "test-token",
		"username": "Nginx UI Test",
	})
	if err != nil {
		t.Fatalf("SendWithConfig() error = %v", err)
	}

	if requestPath != "/hooks/test-token" {
		t.Fatalf("request path = %q, want /hooks/test-token", requestPath)
	}
	if payload.Username != "Nginx UI Test" {
		t.Fatalf("username = %q, want Nginx UI Test", payload.Username)
	}
	if payload.Text != "External Notification Test\n\nMattermost content" {
		t.Fatalf("text = %q, want title and content", payload.Text)
	}
}
