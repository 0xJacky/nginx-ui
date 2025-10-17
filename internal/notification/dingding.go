package notification

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify/service/dingding"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Dingding)
type Dingding struct {
	AccessToken string `json:"access_token" title:"Access Token"`
	Secret      string `json:"secret" title:"Secret (Optional)"`
}

func init() {
	RegisterExternalNotifier("dingding", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		dingdingConfig := &Dingding{}
		err := map2struct.WeakDecode(n.Config, dingdingConfig)
		if err != nil {
			return err
		}
		if dingdingConfig.AccessToken == "" {
			return ErrInvalidNotifierConfig
		}

		// Initialize Dingding service
		dingdingService := dingding.New(&dingding.Config{
			Token:  dingdingConfig.AccessToken,
			Secret: dingdingConfig.Secret,
		})
		return dingdingService.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
