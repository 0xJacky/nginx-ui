package notification

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/bark"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Bark)
type Bark struct {
	DeviceKey string `json:"device_key" title:"Device Key"`
	ServerURL string `json:"server_url" title:"Server URL"`
}

func init() {
	RegisterExternalNotifier("bark", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		barkConfig := &Bark{}
		err := map2struct.WeakDecode(n.Config, barkConfig)
		if err != nil {
			return err
		}
		if barkConfig.DeviceKey == "" && barkConfig.ServerURL == "" {
			return ErrInvalidNotifierConfig
		}
		barkService := bark.NewWithServers(barkConfig.DeviceKey, barkConfig.ServerURL)
		externalNotify := notify.New()
		externalNotify.UseServices(barkService)
		return externalNotify.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
