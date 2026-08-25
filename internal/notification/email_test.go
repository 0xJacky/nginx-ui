package notification

import (
	"net/smtp"
	"reflect"
	"strings"
	"testing"
)

func TestParseEmailRecipientsSupportsCommaSeparatedList(t *testing.T) {
	recipients, err := parseEmailRecipients("first@example.com, Second <second@example.com>")
	if err != nil {
		t.Fatalf("parseEmailRecipients() error = %v", err)
	}

	want := []string{"first@example.com", "second@example.com"}
	if !reflect.DeepEqual(recipients, want) {
		t.Fatalf("parseEmailRecipients() = %#v, want %#v", recipients, want)
	}
}

func TestLoginAuthRejectsUnencryptedNonLocalhostConnection(t *testing.T) {
	auth := &loginAuth{username: "user", password: "secret"}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: false}); err == nil {
		t.Fatal("Start() error = nil, want error for unencrypted non-localhost connection")
	}
}

func TestLoginAuthAllowsLocalhostOrTLSConnection(t *testing.T) {
	auth := &loginAuth{username: "user", password: "secret"}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false}); err != nil {
		t.Fatalf("Start() error = %v, want nil for localhost", err)
	}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: true}); err != nil {
		t.Fatalf("Start() error = %v, want nil for TLS connection", err)
	}
}

func TestLoginAuthRespondsWithUsernameAndPassword(t *testing.T) {
	auth := &loginAuth{username: "user", password: "secret"}
	mechanism, initialResponse, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if mechanism != "LOGIN" || initialResponse != nil {
		t.Fatalf("Start() = (%q, %q), want (LOGIN, nil)", mechanism, initialResponse)
	}

	username, err := auth.Next(nil, true)
	if err != nil || string(username) != "user" {
		t.Fatalf("first Next() = (%q, %v), want (user, nil)", username, err)
	}
	password, err := auth.Next(nil, true)
	if err != nil || string(password) != "secret" {
		t.Fatalf("second Next() = (%q, %v), want (secret, nil)", password, err)
	}
}

func TestSupportsSMTPAuth(t *testing.T) {
	if !supportsSMTPAuth("PLAIN LOGIN", "LOGIN") {
		t.Fatal("supportsSMTPAuth() = false, want true")
	}
	if supportsSMTPAuth("PLAIN LOGIN", "CRAM-MD5") {
		t.Fatal("supportsSMTPAuth() = true, want false")
	}
}

func TestBuildEmailMessageUsesPlainTextByDefault(t *testing.T) {
	message, err := buildEmailMessage("sender@example.com", "receiver@example.com", "Plain subject", "Line 1\nLine 2", false, "")
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("message content type = %q, want text/plain", body)
	}
	if !strings.Contains(body, "Plain subject\n\nLine 1\nLine 2") {
		t.Fatalf("message body = %q, want plain title and content", body)
	}
}

func TestBuildEmailMessageUsesHTMLTemplate(t *testing.T) {
	message, err := buildEmailMessage("sender@example.com", "receiver@example.com", "HTML subject", "<b>Line 1</b>\nLine 2", true, "")
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("message content type = %q, want text/html", body)
	}
	if !strings.Contains(body, "<h1 style=\"margin: 0 0 16px; color: #1677ff;\">HTML subject</h1>") {
		t.Fatalf("message body = %q, want HTML title", body)
	}
	if !strings.Contains(body, "&lt;b&gt;Line 1&lt;/b&gt;<br>\nLine 2") {
		t.Fatalf("message body = %q, want escaped HTML content with line breaks", body)
	}
}

func TestBuildEmailMessageUsesCustomHTMLTemplate(t *testing.T) {
	message, err := buildEmailMessage(
		"sender@example.com",
		"receiver@example.com",
		"Custom subject",
		"Line 1\nLine 2",
		true,
		`<main><h1>{{.Title}}</h1><article>{{.Content}}</article></main>`,
	)
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "<main><h1>Custom subject</h1><article>Line 1<br>\nLine 2</article></main>") {
		t.Fatalf("message body = %q, want custom HTML template", body)
	}
}
