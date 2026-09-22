package notification

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
)

// fakeSource serves the notifier types it was built with.
type fakeSource struct {
	channels []ExternalNotifierChannel
	rejects  error

	mu   sync.Mutex
	sent []string
}

func (s *fakeSource) Channels() []ExternalNotifierChannel { return s.channels }

func (s *fakeSource) Handler(notifierType string) (ExternalNotifierHandlerFunc, bool) {
	for _, channel := range s.channels {
		if channel.Type != notifierType {
			continue
		}
		return func(_ context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.sent = append(s.sent, n.Type+":"+msg.GetTitle(n.Language))
			return nil
		}, true
	}
	return nil, false
}

func (s *fakeSource) Validate(_ context.Context, notifierType string, _ map[string]string) error {
	for _, channel := range s.channels {
		if channel.Type == notifierType {
			return s.rejects
		}
	}
	return nil
}

// withNotifierSource registers a source for one test only.
func withNotifierSource(t *testing.T, source ExternalNotifierSource) {
	t.Helper()
	externalNotifierSourcesMutex.Lock()
	previous := externalNotifierSources
	externalNotifierSources = append(append([]ExternalNotifierSource(nil), previous...), source)
	externalNotifierSourcesMutex.Unlock()
	t.Cleanup(func() {
		externalNotifierSourcesMutex.Lock()
		externalNotifierSources = previous
		externalNotifierSourcesMutex.Unlock()
	})
}

func TestSourceNotifierIsUsedForUnknownTypes(t *testing.T) {
	withTranslationDict(t, nil)
	source := &fakeSource{channels: []ExternalNotifierChannel{{Type: "plugin:mychat", Name: "MyChat"}}}
	withNotifierSource(t, source)

	msg := &ExternalMessage{Notification: &model.Notification{Title: "Hello"}}
	if err := msg.SendWithConfigContext(t.Context(), "plugin:mychat", "en", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(source.sent) != 1 || source.sent[0] != "plugin:mychat:Hello" {
		t.Fatalf("sent = %v", source.sent)
	}

	err := msg.SendWithConfigContext(t.Context(), "plugin:gone", "en", nil)
	if !errors.Is(err, ErrNotifierNotFound) {
		t.Fatalf("err = %v, want ErrNotifierNotFound", err)
	}
}

func TestBuiltinNotifierShadowsASource(t *testing.T) {
	source := &fakeSource{channels: []ExternalNotifierChannel{
		{Type: "bark", Name: "Fake Bark"},
		{Type: "plugin:b", Name: "B"},
		{Type: "plugin:a", Name: "A"},
	}}
	withNotifierSource(t, source)

	channels := ExternalNotifierChannels()
	if len(channels) != 2 || channels[0].Type != "plugin:a" || channels[1].Type != "plugin:b" {
		t.Fatalf("channels = %+v", channels)
	}
	if channels[0].Fields == nil {
		t.Fatal("fields must be an empty list, not null")
	}

	handler, err := externalNotifierHandler(&model.ExternalNotify{Type: "bark"})
	if err != nil || handler == nil {
		t.Fatalf("bark handler = %v, %v", handler, err)
	}
	source.rejects = errors.New("rejected")
	if err = ValidateExternalNotifierConfig(t.Context(), "bark", nil); err != nil {
		t.Fatalf("a built-in type must not reach the source: %v", err)
	}
}

func TestValidateExternalNotifierConfigAsksTheSource(t *testing.T) {
	rejected := errors.New("webhook_url is required")
	withNotifierSource(t, &fakeSource{
		channels: []ExternalNotifierChannel{{Type: "plugin:mychat", Name: "MyChat"}},
		rejects:  rejected,
	})

	if err := ValidateExternalNotifierConfig(t.Context(), "plugin:mychat", nil); !errors.Is(err, rejected) {
		t.Fatalf("err = %v, want the source error", err)
	}
	if err := ValidateExternalNotifierConfig(t.Context(), "plugin:other", nil); err != nil {
		t.Fatalf("an unknown type has no validator: %v", err)
	}
}
