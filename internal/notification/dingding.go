package notification

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify/service/dingding"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(DingTalk)
type DingTalk struct {
	AccessToken string `json:"access_token" title:"Access Token"`
	Secret      string `json:"secret" title:"Secret (Optional)"`
}

func init() {
	RegisterExternalNotifier("dingding", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		dingTalkConfig := &DingTalk{}
		err := map2struct.WeakDecode(n.Config, dingTalkConfig)
		if err != nil {
			return err
		}
		if dingTalkConfig.AccessToken == "" {
			return ErrInvalidNotifierConfig
		}

		// Initialize DingTalk service
		dingTalkService := dingding.New(&dingding.Config{
			Token:  dingTalkConfig.AccessToken,
			Secret: dingTalkConfig.Secret,
		})
		return dingTalkService.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
