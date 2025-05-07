package notification

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify/service/gotify"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Gotify)
type Gotify struct {
	URL      string `json:"url" title:"URL"`
	Token    string `json:"token" title:"Token"`
	Priority int    `json:"priority" title:"Priority"`
}

func init() {
	RegisterExternalNotifier("gotify", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		gotifyConfig := &Gotify{}
		err := map2struct.WeakDecode(n.Config, gotifyConfig)
		if err != nil {
			return err
		}
		if gotifyConfig.URL == "" || gotifyConfig.Token == "" {
			return ErrInvalidNotifierConfig
		}

		gotifyService := gotify.NewWithPriority(gotifyConfig.Token, gotifyConfig.URL, gotifyConfig.Priority)

		return gotifyService.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
