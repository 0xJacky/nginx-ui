package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(WeCom)
type WeCom struct {
	WebhookURL string `json:"webhook_url" title:"Webhook URL"`
}

type wecomMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func init() {
	RegisterExternalNotifier("wecom", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		wecomConfig := &WeCom{}
		err := map2struct.WeakDecode(n.Config, wecomConfig)
		if err != nil {
			return err
		}
		if wecomConfig.WebhookURL == "" {
			return ErrInvalidNotifierConfig
		}

		// Create message payload
		message := wecomMessage{
			MsgType: "text",
		}
		
		title := msg.GetTitle(n.Language)
		content := msg.GetContent(n.Language)
		
		// Combine title and content
		fullMessage := title
		if content != "" {
			fullMessage = fmt.Sprintf("%s\n\n%s", title, content)
		}
		
		message.Text.Content = fullMessage

		// Marshal to JSON
		payload, err := json.Marshal(message)
		if err != nil {
			return err
		}

		// Send HTTP POST request
		req, err := http.NewRequestWithContext(ctx, "POST", wecomConfig.WebhookURL, bytes.NewBuffer(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("weCom webhook returned status code: %d", resp.StatusCode)
		}

		return nil
	})
}
