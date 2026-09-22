package capability

import (
	"context"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// notifySendTimeout bounds one delivery through a vendor.
	notifySendTimeout = 30 * time.Second
	// notifyValidateTimeout bounds a configuration check, which never
	// contacts the vendor.
	notifyValidateTimeout = 10 * time.Second
)

// NotifyHost is the part of the plugin manager the notify capability needs.
type NotifyHost interface {
	// OwnerOf returns the plugin that serves a channel code.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// NotifyChannels lists what every enabled notify plugin offers.
	NotifyChannels() []plugin.NotifyChannelEntry
}

// RegisterNotify offers the channels of every enabled notify plugin as
// external notifier types, next to the built-in ones.
func RegisterNotify(h NotifyHost) {
	notification.RegisterExternalNotifierSource(NewNotifySource(h))
}

// NewNotifySource exposes the notify capability of the plugins of h as a
// notifier source. A channel code is published as PluginType(code).
func NewNotifySource(h NotifyHost) notification.ExternalNotifierSource {
	return &notifySource{host: h}
}

type notifySource struct {
	host NotifyHost
}

// Channels lists every channel once, served by the plugin that owns its code.
func (s *notifySource) Channels() []notification.ExternalNotifierChannel {
	entries := s.host.NotifyChannels()
	channels := make([]notification.ExternalNotifierChannel, 0, len(entries))
	for _, entry := range entries {
		if owner, ok := s.host.OwnerOf(protocol.CapabilityNotify, entry.Channel.Code); !ok || owner != entry.PluginID {
			continue
		}
		fields := configurationFields(entry.Channel.Configuration)
		channel := notification.ExternalNotifierChannel{
			Type:     PluginType(entry.Channel.Code),
			Name:     entry.Channel.Name,
			PluginID: entry.PluginID,
			Fields:   make([]notification.ExternalNotifierField, 0, len(fields)),
		}
		for _, field := range fields {
			channel.Fields = append(channel.Fields, notification.ExternalNotifierField{
				Key:         field.Key,
				Type:        field.Type,
				DisplayName: field.DisplayName,
				HelpText:    field.HelpText,
				Required:    field.Required,
				Secret:      field.Secret,
			})
		}
		channels = append(channels, channel)
	}
	return channels
}

// Handler serves a plugin channel as long as an enabled plugin declares it.
func (s *notifySource) Handler(notifierType string) (notification.ExternalNotifierHandlerFunc, bool) {
	code, ok := pluginCode(notifierType)
	if !ok {
		return nil, false
	}
	if _, ok = s.host.OwnerOf(protocol.CapabilityNotify, code); !ok {
		return nil, false
	}
	return func(ctx context.Context, n *model.ExternalNotify, msg *notification.ExternalMessage) error {
		return s.send(ctx, code, n, msg)
	}, true
}

func (s *notifySource) send(ctx context.Context, code string, n *model.ExternalNotify, msg *notification.ExternalMessage) error {
	// The owner is resolved again, the plugin may have changed since the
	// handler was looked up.
	pluginID, ok := s.host.OwnerOf(protocol.CapabilityNotify, code)
	if !ok {
		return notification.ErrNotifierNotFound
	}

	callCtx, cancel := context.WithTimeout(ctx, notifySendTimeout)
	defer cancel()

	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		return plugin.WrapRPCError(err)
	}

	err = caller.Call(callCtx, protocol.MethodNotifySend, protocol.NotifySendParams{
		Channel:  code,
		Config:   n.Config,
		Title:    msg.GetTitle(n.Language),
		Content:  msg.GetContent(n.Language),
		Severity: msg.GetType(),
	}, nil)
	if err == nil {
		return nil
	}
	if field, message, invalid := invalidConfigField(err); invalid {
		return cosy.WrapErrorWithParams(notification.ErrInvalidNotifierField, field, message)
	}
	return plugin.WrapRPCError(err)
}

// Validate asks the owning plugin to check a configuration. Only a
// CodeInvalidConfig answer rejects it: a plugin that does not implement
// notify.validate or cannot be reached right now has no opinion, and the
// delivery reports the problem later.
func (s *notifySource) Validate(ctx context.Context, notifierType string, config map[string]string) error {
	code, ok := pluginCode(notifierType)
	if !ok {
		return nil
	}
	pluginID, ok := s.host.OwnerOf(protocol.CapabilityNotify, code)
	if !ok {
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, notifyValidateTimeout)
	defer cancel()

	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		logger.Warnf("[plugin:%s] cannot validate notify channel %s: %v", pluginID, code, err)
		return nil
	}

	err = caller.Call(callCtx, protocol.MethodNotifyValidate, protocol.NotifyValidateParams{
		Channel: code,
		Config:  config,
	}, nil)
	switch {
	case err == nil, isUnimplemented(err):
		return nil
	}
	if field, message, invalid := invalidConfigField(err); invalid {
		return cosy.WrapErrorWithParams(notification.ErrInvalidNotifierField, field, message)
	}
	logger.Warnf("[plugin:%s] validate notify channel %s: %s", pluginID, code, rpcMessage(err))
	return nil
}
