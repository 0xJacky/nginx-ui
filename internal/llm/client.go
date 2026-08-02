package llm

import (
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"net/http"
)

// httpDoerOverride replaces the transport every LLM request goes through.
//
// The slot defaults to nil and only internal/demo fills it, so a production
// binary always talks to the configured provider. A public demo has no API key
// to spend and no business making outbound calls on a visitor's behalf, but an
// assistant panel that errors on every message is worse than one that answers.
var httpDoerOverride openai.HTTPDoer

// SetHTTPDoer installs a transport override. Call once, at boot.
func SetHTTPDoer(doer openai.HTTPDoer) {
	httpDoerOverride = doer
}

func GetClient() (*openai.Client, error) {
	var config openai.ClientConfig
	baseURL := settings.OpenAISettings.GetBaseURL()
	if openai.APIType(settings.OpenAISettings.APIType) == openai.APITypeAzure {
		config = openai.DefaultAzureConfig(settings.OpenAISettings.Token, baseURL)
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

	if baseURL != "" {
		config.BaseURL = baseURL
	}

	// Last, so it wins over the proxy client configured above: an override
	// exists precisely to stop the request leaving the process.
	if doer := httpDoerOverride; doer != nil {
		config.HTTPClient = doer
	}

	return openai.NewClientWithConfig(config), nil
}
