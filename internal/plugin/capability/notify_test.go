package capability

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

const notifyPluginID = "io.github.example.chat"

func notifyHost(caller *fakeCaller) *capabilityHost {
	h := newCapabilityHost()
	h.callers[notifyPluginID] = caller
	h.channels = []plugin.NotifyChannelEntry{
		{PluginID: notifyPluginID, Channel: protocol.NotifyChannel{
			Code: "mychat",
			Name: "MyChat",
			Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
				{Key: "webhook_url", DisplayName: "Webhook URL", Required: true},
				{Key: "token", DisplayName: "Token", Secret: true},
			}},
		}},
		// A second plugin declaring the same code does not own it.
		{PluginID: "io.github.other.chat", Channel: protocol.NotifyChannel{Code: "mychat", Name: "Shadowed"}},
	}
	h.own(protocol.CapabilityNotify, "mychat", notifyPluginID)
	return h
}

func TestNotifyChannelsListTheOwnedCodes(t *testing.T) {
	channels := NewNotifySource(notifyHost(newFakeCaller())).Channels()
	if len(channels) != 1 {
		t.Fatalf("channels = %+v, want one", channels)
	}
	channel := channels[0]
	if channel.Type != "plugin:mychat" || channel.Name != "MyChat" || channel.PluginID != notifyPluginID {
		t.Fatalf("channel = %+v", channel)
	}
	if len(channel.Fields) != 2 || !channel.Fields[0].Required || !channel.Fields[1].Secret {
		t.Fatalf("fields = %+v", channel.Fields)
	}
}

func TestNotifyHandlerSendsThroughThePlugin(t *testing.T) {
	caller := newFakeCaller()
	host := notifyHost(caller)
	source := NewNotifySource(host)

	if _, ok := source.Handler("bark"); ok {
		t.Fatal("a built-in type must not be served by the plugin source")
	}
	if _, ok := source.Handler("plugin:unknown"); ok {
		t.Fatal("a code nobody owns must not be served")
	}

	handler, ok := source.Handler("plugin:mychat")
	if !ok {
		t.Fatal("the owned channel has no handler")
	}
	n := &model.ExternalNotify{Type: "plugin:mychat", Language: "en", Config: map[string]string{"webhook_url": "https://chat.example"}}
	msg := &notification.ExternalMessage{Notification: &model.Notification{
		Type: model.NotificationWarning, Title: "Disk almost full", Content: "90% used",
	}}
	if err := handler(t.Context(), n, msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	calls := caller.methodCalls(protocol.MethodNotifySend)
	if len(calls) != 1 {
		t.Fatalf("notify.send calls = %d, want 1", len(calls))
	}
	var params protocol.NotifySendParams
	if err := json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Channel != "mychat" || params.Title != "Disk almost full" || params.Content != "90% used" ||
		params.Severity != protocol.NotifySeverityWarning || params.Config["webhook_url"] != "https://chat.example" {
		t.Fatalf("params = %+v", params)
	}
	if host.releaseCount() != 1 {
		t.Fatalf("releases = %d, want 1", host.releaseCount())
	}
}

func TestNotifySendMapsTheErrors(t *testing.T) {
	caller := newFakeCaller()
	caller.errs[protocol.MethodNotifySend] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "webhook_url is required",
		Data: map[string]any{"field": "webhook_url"},
	}
	source := NewNotifySource(notifyHost(caller))
	handler, _ := source.Handler("plugin:mychat")

	msg := &notification.ExternalMessage{Notification: &model.Notification{Title: "t"}}
	err := handler(t.Context(), &model.ExternalNotify{Type: "plugin:mychat"}, msg)
	assertNotifierField(t, err, "webhook_url")

	caller.errs[protocol.MethodNotifySend] = &protocol.Error{Code: protocol.CodeInternalError, Message: "vendor is down"}
	err = handler(t.Context(), &model.ExternalNotify{Type: "plugin:mychat"}, msg)
	if !errors.Is(err, plugin.ErrRPC) {
		t.Fatalf("err = %v, want the plugin rpc error", err)
	}
}

func assertNotifierField(t *testing.T, err error, field string) {
	t.Helper()
	cErr, ok := errors.AsType[*cosy.Error](err)
	if !ok {
		t.Fatalf("err = %v, want a cosy error", err)
	}
	wantErr, _ := errors.AsType[*cosy.Error](notification.ErrInvalidNotifierField)
	if cErr.Code != wantErr.Code || len(cErr.Params) == 0 || cErr.Params[0] != field {
		t.Fatalf("err = %+v, want code %d for field %s", cErr, wantErr.Code, field)
	}
}

func TestNotifyValidate(t *testing.T) {
	caller := newFakeCaller()
	host := notifyHost(caller)
	source := NewNotifySource(host)

	// Well-formed.
	if err := source.Validate(t.Context(), "plugin:mychat", map[string]string{"webhook_url": "x"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	calls := caller.methodCalls(protocol.MethodNotifyValidate)
	if len(calls) != 1 {
		t.Fatalf("notify.validate calls = %d", len(calls))
	}

	// Rejected with the field.
	caller.errs[protocol.MethodNotifyValidate] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "bad url", Data: map[string]any{"field": "webhook_url"},
	}
	assertNotifierField(t, source.Validate(t.Context(), "plugin:mychat", nil), "webhook_url")

	// Unimplemented, failing or unreachable plugins have no opinion.
	for _, err := range []error{
		&protocol.Error{Code: protocol.CodeUnsupported, Message: "unsupported"},
		&protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown"},
		&protocol.Error{Code: protocol.CodeInternalError, Message: "boom"},
	} {
		caller.errs[protocol.MethodNotifyValidate] = err
		if got := source.Validate(t.Context(), "plugin:mychat", nil); got != nil {
			t.Fatalf("validate with %v = %v, want nil", err, got)
		}
	}
	host.acquireErr = errors.New("cannot start")
	if err := source.Validate(t.Context(), "plugin:mychat", nil); err != nil {
		t.Fatalf("validate of an unreachable plugin = %v, want nil", err)
	}

	// Types the source does not serve are not checked at all.
	if err := source.Validate(t.Context(), "telegram", nil); err != nil {
		t.Fatalf("validate of a built-in type = %v", err)
	}
}
