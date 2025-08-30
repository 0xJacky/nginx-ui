package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/map2struct"
	"net/http"
	"strconv"
)

const (
	DEFAULT_NTFY_PRIORITY = 3
	DEFAULT_NTFY_ICON     = "https://nginxui.com/assets/logo.svg"
)

// @external_notifier(Ntfy)
type Ntfy struct {
	ServerURL string `json:"server_url" title:"Server URL"`
	Topic     string `json:"topic" title:"Topic"`
	Priority  string `json:"priority" title:"Priority"`
	Tags      string `json:"tags" title:"Tags"`
	Click     string `json:"click" title:"Click URL"`
	Actions   string `json:"actions" title:"Actions"`
	Username  string `json:"username" title:"Username"`
	Password  string `json:"password" title:"Password"`
	Token     string `json:"token" title:"Token"`
}

type NtfyMessage struct {
	Topic    string        `json:"topic,omitempty"`
	Message  string        `json:"message,omitempty"`
	Title    string        `json:"title,omitempty"`
	Priority int           `json:"priority,omitempty"`
	Tags     []string      `json:"tags,omitempty"`
	Click    string        `json:"click,omitempty"`
	Actions  []interface{} `json:"actions,omitempty"`
	Icon     string        `json:"icon,omitempty"`
}

func init() {
	RegisterExternalNotifier("ntfy", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		ntfyConfig := &Ntfy{}
		err := map2struct.WeakDecode(n.Config, ntfyConfig)
		if err != nil {
			return err
		}
		if ntfyConfig.ServerURL == "" || ntfyConfig.Topic == "" {
			return ErrInvalidNotifierConfig
		}

		// Convert priority string to int
		priority := DEFAULT_NTFY_PRIORITY
		if ntfyConfig.Priority != "" {
			p, err := strconv.Atoi(ntfyConfig.Priority)
			if err != nil || p < 1 || p > 5 {
				return fmt.Errorf("invalid priority: %w", err)
			}
			priority = p
		}

		// Prepare the message
		ntfyMsg := NtfyMessage{
			Topic:    ntfyConfig.Topic,
			Message:  msg.GetContent(n.Language),
			Title:    msg.GetTitle(n.Language),
			Priority: priority,
			Icon:     DEFAULT_NTFY_ICON,
			Click:    ntfyConfig.Click,
		}

		// Add tags if provided
		if ntfyConfig.Tags != "" {
			var tags []string
			if err := json.Unmarshal([]byte(ntfyConfig.Tags), &tags); err != nil {
				return fmt.Errorf("invalid tags: %w", err)
			}
			ntfyMsg.Tags = tags
		}

		// Add actions if provided
		if ntfyConfig.Actions != "" {
			var actions []interface{}
			if err := json.Unmarshal([]byte(ntfyConfig.Actions), &actions); err != nil {
				return fmt.Errorf("invalid actions: %w", err)
			}
			ntfyMsg.Actions = actions
		}

		// Create HTTP request
		jsonData, err := json.Marshal(ntfyMsg)
		if err != nil {
			return fmt.Errorf("failed to marshal ntfy message: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", ntfyConfig.ServerURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Nginx-UI")
		if ntfyConfig.Token != "" {
			req.Header.Set("Authorization", "Bearer "+ntfyConfig.Token)
		} else if ntfyConfig.Username != "" && ntfyConfig.Password != "" {
			req.SetBasicAuth(ntfyConfig.Username, ntfyConfig.Password)
		}

		// Send request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send ntfy request: %w", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("ntfy request failed with status: %d", resp.StatusCode)
		}

		return nil
	})
}
