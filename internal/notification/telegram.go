package notification

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/nikoksr/notify/service/telegram"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Telegram)
type Telegram struct {
	BotToken string `json:"bot_token" title:"Bot Token"`
	ChatID   string `json:"chat_id" title:"Chat ID"`
}

func init() {
	RegisterExternalNotifier("telegram", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		telegramConfig := &Telegram{}
		err := map2struct.WeakDecode(n.Config, telegramConfig)
		if err != nil {
			return err
		}
		if telegramConfig.BotToken == "" || telegramConfig.ChatID == "" {
			return ErrInvalidNotifierConfig
		}

		telegramService, err := telegram.New(telegramConfig.BotToken)
		if err != nil {
			return err
		}

		// ChatID must be an integer for telegram service
		chatIDInt, err := strconv.ParseInt(telegramConfig.ChatID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid Telegram Chat ID '%s': %w", telegramConfig.ChatID, err)
		}

		// Check if chatIDInt is 0, which might indicate an empty or invalid input was parsed
		if chatIDInt == 0 {
			return errors.New("invalid Telegram Chat ID: cannot be zero")
		}

		telegramService.AddReceivers(chatIDInt)
		
		return telegramService.Send(ctx, msg.GetTitle(n.Language), msg.GetContent(n.Language))
	})
}
