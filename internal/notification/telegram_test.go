package notification

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type telegramRoundTripFunc func(*http.Request) (*http.Response, error)

func (f telegramRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestTelegramNotifierSendsMessageToConfiguredTopic(t *testing.T) {
	originalFactory := newTelegramBotAPI
	t.Cleanup(func() { newTelegramBotAPI = originalFactory })

	var sentForm url.Values
	newTelegramBotAPI = func(token string, _ *http.Client) (*tgbotapi.BotAPI, error) {
		return &tgbotapi.BotAPI{
			Token: token,
			Client: &http.Client{Transport: telegramRoundTripFunc(
				func(request *http.Request) (*http.Response, error) {
					if err := request.ParseForm(); err != nil {
						t.Fatalf("ParseForm() error = %v", err)
					}
					sentForm = request.PostForm
					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(
							`{"ok":true,"result":{}}`,
						)),
						Header: make(http.Header),
					}, nil
				},
			)},
		}, nil
	}

	message := &ExternalMessage{Notification: &model.Notification{
		Title:   "Sync Config Success",
		Content: "Configuration synchronized",
	}}
	err := message.SendWithConfigContext(context.Background(), "telegram", "en", map[string]string{
		"bot_token":         "test-token",
		"chat_id":           "-1001234567890",
		"message_thread_id": "42",
	})
	if err != nil {
		t.Fatalf("SendWithConfigContext() error = %v", err)
	}

	if got := sentForm.Get("chat_id"); got != "-1001234567890" {
		t.Fatalf("chat_id = %q", got)
	}
	if got := sentForm.Get("message_thread_id"); got != "42" {
		t.Fatalf("message_thread_id = %q", got)
	}
	if got := sentForm.Get("parse_mode"); got != tgbotapi.ModeHTML {
		t.Fatalf("parse_mode = %q", got)
	}
	if got := sentForm.Get("text"); got != "Sync Config Success\nConfiguration synchronized" {
		t.Fatalf("text = %q", got)
	}
}

func TestTelegramNotifierRejectsInvalidMessageThreadID(t *testing.T) {
	message := &ExternalMessage{Notification: &model.Notification{}}
	err := message.SendWithConfig("telegram", "en", map[string]string{
		"bot_token":         "test-token",
		"chat_id":           "-1001234567890",
		"message_thread_id": "not-a-number",
	})
	if err == nil || !strings.Contains(err.Error(), "Message Thread ID") {
		t.Fatalf("error = %v", err)
	}
}
