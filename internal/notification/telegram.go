package notification

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/0xJacky/Nginx-UI/model"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Telegram)
type Telegram struct {
	BotToken        string `json:"bot_token" title:"Bot Token"`
	ChatID          string `json:"chat_id" title:"Chat ID"`
	MessageThreadID string `json:"message_thread_id" title:"Message Thread ID"`
	HTTPProxy       string `json:"http_proxy" title:"HTTP Proxy"`
}

var newTelegramBotAPI = tgbotapi.NewBotAPIWithClient

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

		client := http.DefaultClient
		if telegramConfig.HTTPProxy != "" {
			proxyURL, err := url.Parse(telegramConfig.HTTPProxy)
			if err != nil {
				return err
			}
			client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
		}

		// ChatID must be an integer for telegram service
		chatIDInt, err := strconv.ParseInt(telegramConfig.ChatID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid Telegram Chat ID '%s': %w", telegramConfig.ChatID, err)
		}

		// Check if chatIDInt is 0, which might indicate an empty or invalid input was parsed
		if chatIDInt == 0 {
			return ErrTelegramChatIDZero
		}

		var messageThreadID int64
		if telegramConfig.MessageThreadID != "" {
			messageThreadID, err = strconv.ParseInt(
				telegramConfig.MessageThreadID, 10, 64,
			)
			if err != nil || messageThreadID <= 0 {
				return fmt.Errorf(
					"invalid Telegram Message Thread ID %q",
					telegramConfig.MessageThreadID,
				)
			}
		}

		botAPI, err := newTelegramBotAPI(telegramConfig.BotToken, client)
		if err != nil {
			return err
		}

		params := url.Values{
			"chat_id":    {strconv.FormatInt(chatIDInt, 10)},
			"text":       {msg.GetTitle(n.Language) + "\n" + msg.GetContent(n.Language)},
			"parse_mode": {tgbotapi.ModeHTML},
		}
		if messageThreadID > 0 {
			params.Set("message_thread_id", strconv.FormatInt(messageThreadID, 10))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, err = botAPI.MakeRequest("sendMessage", params)
			if err != nil {
				return fmt.Errorf("send message to chat %d: %w", chatIDInt, err)
			}
			return nil
		}
	})
}
