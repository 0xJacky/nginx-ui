package notification

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify/service/lark"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Lark Custom)
type LarkCustom struct {
	Domain    string `json:"domain" title:"Domain"`
	AppID     string `json:"app_id" title:"App ID"`
	AppSecret string `json:"app_secret" title:"App Secret"`
	OpenID    string `json:"open_id" title:"Open ID"`
	UserID    string `json:"user_id" title:"User ID"`
	UnionID   string `json:"union_id" title:"Union ID"`
	Email     string `json:"email" title:"Email"`
	ChatID    string `json:"chat_id" title:"Chat ID"`
}

func init() {
	RegisterExternalNotifier("lark_custom", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		larkCustomConfig := &LarkCustom{}
		err := map2struct.WeakDecode(n.Config, larkCustomConfig)
		if err != nil {
			return err
		}
		if larkCustomConfig.AppID == "" || larkCustomConfig.AppSecret == "" {
			return ErrInvalidNotifierConfig
		}

		larkCustomAppService := lark.NewCustomAppService(larkCustomConfig.AppID, larkCustomConfig.AppSecret)
		larkCustomAppService.AddReceivers(
			lark.OpenID(larkCustomConfig.OpenID),
			lark.UserID(larkCustomConfig.UserID),
			lark.UnionID(larkCustomConfig.UnionID),
			lark.Email(larkCustomConfig.Email),
			lark.ChatID(larkCustomConfig.ChatID),
		)

		if larkCustomConfig.Domain != "" {
			larkCustomAppService.AddReceivers(
				lark.Domain(larkCustomConfig.Domain),
			)
		}

		return larkCustomAppService.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
