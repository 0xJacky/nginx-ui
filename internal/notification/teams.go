package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/map2struct"
)

var (
	teamsTokenURLTemplate = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
)

// @external_notifier(Microsoft Teams（Power Automate webhook）)
type Teams struct {
	TenantID           string `json:"tenant_id" title:"Tenant ID"`
	ClientID           string `json:"client_id" title:"Client ID"`
	ClientSecret       string `json:"client_secret" title:"Client Secret"`
	WorkflowWebhookURL string `json:"workflow_webhook_url" title:"Workflow Webhook URL"`
	MessageBody        string `json:"message_body" title:"Message Body (JSON)"`
}

type teamsOAuthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func init() {
	RegisterExternalNotifier("teams", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		teamsConfig := &Teams{}
		err := map2struct.WeakDecode(n.Config, teamsConfig)
		if err != nil {
			return err
		}

		tenantID := strings.TrimSpace(teamsConfig.TenantID)
		if tenantID == "" {
			// Backward compatibility for earlier configs that used organization_id.
			tenantID = strings.TrimSpace(n.Config["organization_id"])
		}

		workflowWebhookURL := strings.TrimSpace(teamsConfig.WorkflowWebhookURL)
		if workflowWebhookURL == "" {
			// Backward compatibility for earlier configs that used webhook_url.
			workflowWebhookURL = strings.TrimSpace(n.Config["webhook_url"])
		}

		if tenantID == "" || teamsConfig.ClientID == "" || teamsConfig.ClientSecret == "" || workflowWebhookURL == "" {
			return ErrInvalidNotifierConfig
		}

		_, err = url.ParseRequestURI(workflowWebhookURL)
		if err != nil {
			return ErrInvalidNotifierConfig
		}

		httpClient, err := teamsHTTPClient()
		if err != nil {
			return err
		}

		accessToken, err := teamsFetchAccessToken(ctx, httpClient, tenantID, teamsConfig)
		if err != nil {
			return err
		}

		payload, err := buildTeamsWorkflowPayload(
			msg.GetTitle(n.Language),
			msg.GetContent(n.Language),
			teamsConfig.MessageBody,
		)
		if err != nil {
			return err
		}

		return teamsSendWorkflowWebhook(ctx, httpClient, accessToken, workflowWebhookURL, payload)
	})
}

func teamsHTTPClient() (*http.Client, error) {
	if strings.TrimSpace(settings.HTTPSettings.HTTPProxy) == "" {
		return http.DefaultClient, nil
	}

	httpTransport, err := transport.NewTransport(transport.WithProxy(settings.HTTPSettings.HTTPProxy))
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: httpTransport}, nil
}

func buildTeamsWorkflowPayload(title, content, messageBody string) (map[string]any, error) {
	body := strings.TrimSpace(messageBody)
	var bodyBlocks []any

	if body == "" {
		bodyBlocks = defaultTeamsWorkflowBody(title, content)
	} else {
		if err := json.Unmarshal([]byte(body), &bodyBlocks); err != nil {
			return nil, ErrInvalidNotifierConfig
		}
		bodyBlocks = replaceTeamsPayloadPlaceholders(bodyBlocks, title, content).([]any)
		bodyBlocks = normalizeAdaptiveCardBodyColors(bodyBlocks).([]any)
	}

	return map[string]any{
		"type": "message",
		"attachments": []any{
			map[string]any{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]any{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": "1.4",
					"body":    bodyBlocks,
				},
			},
		},
	}, nil
}

func replaceTeamsPayloadPlaceholders(value any, title, content string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			typed[key] = replaceTeamsPayloadPlaceholders(item, title, content)
		}
		return typed
	case []any:
		for index, item := range typed {
			typed[index] = replaceTeamsPayloadPlaceholders(item, title, content)
		}
		return typed
	case string:
		replaced := strings.ReplaceAll(typed, "{{title}}", title)
		replaced = strings.ReplaceAll(replaced, "{{content}}", content)
		return replaced
	default:
		return value
	}
}

func normalizeAdaptiveCardBodyColors(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "color" {
				if colorValue, ok := item.(string); ok {
					typed[key] = normalizeAdaptiveCardColor(colorValue)
					continue
				}
			}
			typed[key] = normalizeAdaptiveCardBodyColors(item)
		}
		return typed
	case []any:
		for index, item := range typed {
			typed[index] = normalizeAdaptiveCardBodyColors(item)
		}
		return typed
	default:
		return value
	}
}

func normalizeAdaptiveCardColor(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))

	colorMap := map[string]string{
		"default":   "Default",
		"dark":      "Dark",
		"light":     "Light",
		"accent":    "Accent",
		"good":      "Good",
		"warning":   "Warning",
		"attention": "Attention",
		"blue":      "Accent",
		"info":      "Accent",
		"primary":   "Accent",
		"green":     "Good",
		"success":   "Good",
		"ok":        "Good",
		"normal":    "Good",
		"yellow":    "Warning",
		"orange":    "Warning",
		"warn":      "Warning",
		"red":       "Attention",
		"danger":    "Attention",
		"error":     "Attention",
		"alert":     "Attention",
		"critical":  "Attention",
		"gray":      "Default",
		"grey":      "Default",
		"muted":     "Default",
		"secondary": "Default",
		"black":     "Dark",
		"white":     "Light",
		"正常":        "Good",
		"警告":        "Warning",
		"异常":        "Attention",
		"告警":        "Attention",
	}

	if normalized, ok := colorMap[key]; ok {
		return normalized
	}

	return raw
}

func defaultTeamsWorkflowBody(title, content string) []any {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Nginx UI Notification"
	}

	return []any{
		map[string]any{
			"type":   "TextBlock",
			"text":   title,
			"weight": "Bolder",
			"size":   "Large",
			"color":  "Attention",
		},
		map[string]any{
			"type": "FactSet",
			"facts": []any{
				map[string]any{"title": "Content", "value": content},
			},
		},
	}
}

func teamsFetchAccessToken(ctx context.Context, httpClient *http.Client, tenantID string, config *Teams) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	clientID := strings.TrimSpace(config.ClientID)
	clientSecret := strings.TrimSpace(config.ClientSecret)
	// Keep the double slash: some Teams Workflows tenants require this exact audience value.
	scope := "https://service.flow.microsoft.com//.default"

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", scope)
	form.Set("grant_type", "client_credentials")

	tokenURL := fmt.Sprintf(teamsTokenURLTemplate, url.PathEscape(tenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Nginx-UI")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("teams token endpoint returned status code: %d (tenant_id=%s scope=%s body=%s)", resp.StatusCode, tenantID, scope, strings.TrimSpace(string(body)))
	}

	tokenResp := &teamsOAuthTokenResponse{}
	if err := json.Unmarshal(body, tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("teams token request failed: %s (%s)", tokenResp.Error, tokenResp.ErrorDescription)
	}

	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("teams token response missing access token")
	}

	return tokenResp.AccessToken, nil
}

func teamsSendWorkflowWebhook(
	ctx context.Context,
	httpClient *http.Client,
	accessToken, workflowWebhookURL string,
	payload map[string]any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, workflowWebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Nginx-UI")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("teams workflow webhook returned status code: %d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}
