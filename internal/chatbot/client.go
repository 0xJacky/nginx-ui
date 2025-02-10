package chatbot

import (
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"net/http"
)

func GetClient() (*openai.Client, error) {
	var config openai.ClientConfig
	if openai.APIType(settings.OpenAISettings.APIType) == openai.APITypeAzure {
		config = openai.DefaultAzureConfig(settings.OpenAISettings.Token, settings.OpenAISettings.BaseUrl)
	} else {
		config = openai.DefaultConfig(settings.OpenAISettings.Token)
	}

	if settings.OpenAISettings.Proxy != "" {
		t, err := transport.NewTransport(transport.WithProxy(settings.OpenAISettings.Proxy))
		if err != nil {
			return nil, err
		}
		config.HTTPClient = &http.Client{
			Transport: t,
		}
	}

	if settings.OpenAISettings.BaseUrl != "" {
		config.BaseURL = settings.OpenAISettings.BaseUrl
	}

	return openai.NewClientWithConfig(config), nil
}
