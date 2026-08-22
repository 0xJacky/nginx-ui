package notification

import (
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

func TestBuildEmailMessageUsesPlainTextByDefault(t *testing.T) {
	message, err := buildEmailMessage("sender@example.com", "receiver@example.com", "Plain subject", "Line 1\nLine 2", false)
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
	message, err := buildEmailMessage("sender@example.com", "receiver@example.com", "HTML subject", "<b>Line 1</b>\nLine 2", true)
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("message content type = %q, want text/html", body)
	}
	if !strings.Contains(body, "<h2>HTML subject</h2>") {
		t.Fatalf("message body = %q, want HTML title", body)
	}
	if !strings.Contains(body, "&lt;b&gt;Line 1&lt;/b&gt;<br>\nLine 2") {
		t.Fatalf("message body = %q, want escaped HTML content with line breaks", body)
	}
}
