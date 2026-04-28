package notification

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/pofile"
)

func withTranslationDict(t *testing.T, dict map[string]pofile.Dict) {
	t.Helper()

	original := translation.Dict
	translation.Dict = dict

	t.Cleanup(func() {
		translation.Dict = original
	})
}

func TestExternalMessageGetContentTranslatesAndInterpolates(t *testing.T) {
	withTranslationDict(t, map[string]pofile.Dict{
		"zh_CN": {
			"Certificate %{name} renewed successfully": "CERT %{name} OK",
		},
		"en": {},
	})

	msg := &ExternalMessage{
		Notification: &model.Notification{
			Content: "Certificate %{name} renewed successfully",
			Details: map[string]any{
				"name": "example.com",
			},
		},
	}

	got := msg.GetContent("zh_CN")
	want := "CERT example.com OK"
	if got != want {
		t.Fatalf("GetContent() = %q, want %q", got, want)
	}
}

func TestExternalMessageGetContentInterpolatesMissingTranslationKey(t *testing.T) {
	withTranslationDict(t, map[string]pofile.Dict{
		"en": {},
	})

	msg := &ExternalMessage{
		Notification: &model.Notification{
			Content: "Certificate %{name} renewal failed: %{error}",
			Details: map[string]any{
				"name":  "example.com",
				"error": "timeout",
			},
		},
	}

	got := msg.GetContent("en")
	want := "Certificate example.com renewal failed: timeout"
	if got != want {
		t.Fatalf("GetContent() = %q, want %q", got, want)
	}
}
