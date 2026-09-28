package llm

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useSavedOpenAISettings(t *testing.T, saved *settings.OpenAI) {
	t.Helper()
	original := settings.OpenAISettings
	settings.OpenAISettings = saved
	t.Cleanup(func() {
		settings.OpenAISettings = original
	})
}

func TestStoredTokenForSavedEndpoint(t *testing.T) {
	useSavedOpenAISettings(t, &settings.OpenAI{
		Provider: settings.OpenAIProviderMiniMax,
		Token:    "saved-token",
		APIType:  string(openai.APITypeOpenAI),
	})

	// The settings page shows the provider default as the base URL, so an
	// explicit copy of it is still the saved endpoint.
	token, err := storedTokenFor(&settings.OpenAI{
		Provider: settings.OpenAIProviderMiniMax,
		BaseUrl:  settings.MiniMaxGlobalOpenAIURL + "/",
		APIType:  string(openai.APITypeOpenAI),
	})

	require.NoError(t, err)
	assert.Equal(t, "saved-token", token)
}

func TestStoredTokenForRefusesOtherEndpoints(t *testing.T) {
	useSavedOpenAISettings(t, &settings.OpenAI{
		BaseUrl: "https://api.openai.com/v1",
		Token:   "saved-token",
		APIType: string(openai.APITypeOpenAI),
	})

	tests := map[string]*settings.OpenAI{
		"different base url": {
			BaseUrl: "https://collector.example.com/v1",
			APIType: string(openai.APITypeOpenAI),
		},
		"different proxy": {
			BaseUrl: "https://api.openai.com/v1",
			Proxy:   "http://proxy.example.com:8080",
			APIType: string(openai.APITypeOpenAI),
		},
		"different api type": {
			BaseUrl: "https://api.openai.com/v1",
			APIType: string(openai.APITypeAzure),
		},
	}

	for name, options := range tests {
		t.Run(name, func(t *testing.T) {
			token, err := storedTokenFor(options)
			assert.ErrorIs(t, err, llm.ErrModelListTokenRequired)
			assert.Empty(t, token)
		})
	}
}

func TestStoredTokenForWithoutSavedToken(t *testing.T) {
	useSavedOpenAISettings(t, &settings.OpenAI{
		BaseUrl: "http://localhost:11434/v1",
		APIType: string(openai.APITypeOpenAI),
	})

	token, err := storedTokenFor(&settings.OpenAI{
		BaseUrl: "http://192.168.1.20:11434/v1",
		APIType: string(openai.APITypeOpenAI),
	})

	require.NoError(t, err)
	assert.Empty(t, token)
}
