package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/map2struct"
)

// @external_notifier(Mattermost)
type Mattermost struct {
	URL      string `json:"url" title:"URL"`
	Token    string `json:"token" title:"Token"`
	Username string `json:"username" title:"Username"`
}

type mattermostMessage struct {
	Text     string `json:"text"`
	Username string `json:"username,omitempty"`
}

func buildMattermostWebhookURL(baseURL, token string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	token = strings.Trim(strings.TrimSpace(token), "/")

	if strings.HasSuffix(baseURL, "/hooks") {
		return fmt.Sprintf("%s/%s", baseURL, token)
	}

	return fmt.Sprintf("%s/hooks/%s", baseURL, token)
}

func init() {
	RegisterExternalNotifier("mattermost", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		mattermostConfig := &Mattermost{}
		err := map2struct.WeakDecode(n.Config, mattermostConfig)
		if err != nil {
			return err
		}
		if mattermostConfig.URL == "" || mattermostConfig.Token == "" {
			return ErrInvalidNotifierConfig
		}

		title := msg.GetTitle(n.Language)
		content := msg.GetContent(n.Language)
		text := title
		if content != "" {
			text = fmt.Sprintf("%s\n\n%s", title, content)
		}

		payload, err := json.Marshal(mattermostMessage{
			Text:     text,
			Username: mattermostConfig.Username,
		})
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", buildMattermostWebhookURL(mattermostConfig.URL, mattermostConfig.Token), bytes.NewBuffer(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Nginx-UI")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("mattermost webhook returned status code: %d", resp.StatusCode)
		}

		return nil
	})
}
