package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenAIGetProvider(t *testing.T) {
	t.Run("defaults to openai", func(t *testing.T) {
		cfg := &OpenAI{}

		assert.Equal(t, OpenAIProviderOpenAI, cfg.GetProvider())
	})

	t.Run("infers atlas cloud from base url", func(t *testing.T) {
		cfg := &OpenAI{BaseUrl: "https://api.atlascloud.ai/v1/"}

		assert.Equal(t, OpenAIProviderAtlasCloud, cfg.GetProvider())
	})

	t.Run("infers minimax from global base url", func(t *testing.T) {
		cfg := &OpenAI{BaseUrl: "https://api.minimax.io/v1/"}

		assert.Equal(t, OpenAIProviderMiniMax, cfg.GetProvider())
	})

	t.Run("infers minimax from cn base url", func(t *testing.T) {
		cfg := &OpenAI{BaseUrl: "https://api.minimaxi.com/v1/"}

		assert.Equal(t, OpenAIProviderMiniMax, cfg.GetProvider())
	})
}

func TestOpenAIGetBaseURL(t *testing.T) {
	t.Run("returns atlas default base url from provider preset", func(t *testing.T) {
		cfg := &OpenAI{Provider: OpenAIProviderAtlasCloud}

		assert.Equal(t, AtlasCloudBaseURL, cfg.GetBaseURL())
	})

	t.Run("returns minimax default base url from provider preset", func(t *testing.T) {
		cfg := &OpenAI{Provider: OpenAIProviderMiniMax}

		assert.Equal(t, MiniMaxGlobalOpenAIURL, cfg.GetBaseURL())
	})

	t.Run("normalizes custom base url", func(t *testing.T) {
		cfg := &OpenAI{Provider: OpenAIProviderCustom, BaseUrl: "https://example.com/v1/"}

		assert.Equal(t, "https://example.com/v1", cfg.GetBaseURL())
	})
}
