package notification

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify/service/lark"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Lark)
type Lark struct {
	WebhookURL string `json:"webhook_url" title:"Webhook URL"`
}

func init() {
	RegisterExternalNotifier("lark", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		larkConfig := &Lark{}
		err := map2struct.WeakDecode(n.Config, larkConfig)
		if err != nil {
			return err
		}
		if larkConfig.WebhookURL == "" {
			return ErrInvalidNotifierConfig
		}

		larkService := lark.NewWebhookService(larkConfig.WebhookURL)
		return larkService.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
