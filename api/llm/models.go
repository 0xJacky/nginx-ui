package llm

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type listModelsPayload struct {
	Provider string `json:"provider" binding:"omitempty,oneof=openai atlas_cloud minimax custom"`
	BaseUrl  string `json:"base_url" binding:"omitempty,url"`
	Token    string `json:"token" binding:"omitempty,safety_text"`
	Proxy    string `json:"proxy" binding:"omitempty,url"`
	APIType  string `json:"api_type" binding:"omitempty,oneof=OPEN_AI AZURE"`
}

// ListModels lists the models of the provider described in the payload. The
// payload carries the values on the settings form, which may not be saved yet,
// so a user can check a connection before committing to it.
func ListModels(c *gin.Context) {
	var payload listModelsPayload
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	options := &settings.OpenAI{
		Provider: payload.Provider,
		BaseUrl:  payload.BaseUrl,
		Token:    payload.Token,
		Proxy:    payload.Proxy,
		APIType:  payload.APIType,
	}

	if payload.Token == settings.RedactedSensitiveValue {
		token, err := storedTokenFor(options)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, err)
			return
		}
		options.Token = token
	}

	models, err := llm.ListModels(c.Request.Context(), options)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
	})
}

// storedTokenFor resolves the redacted token placeholder to the saved token.
// The saved token is only sent to the saved endpoint through the saved proxy:
// otherwise anyone who can open the settings page could point the base URL at
// a server of their own and read the token from the request.
func storedTokenFor(options *settings.OpenAI) (string, error) {
	saved := settings.OpenAISettings
	if saved.Token == "" {
		return "", nil
	}

	if options.GetBaseURL() != saved.GetBaseURL() ||
		options.Proxy != saved.Proxy ||
		options.APIType != saved.APIType {
		return "", llm.ErrModelListTokenRequired
	}

	return saved.Token, nil
}
