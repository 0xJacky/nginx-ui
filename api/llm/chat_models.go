package llm

import (
	"errors"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type chatModelPayload struct {
	Name           string                  `json:"name" binding:"required,safety_text"`
	ThinkingPreset string                  `json:"thinking_preset"`
	ThinkingParams model.LLMThinkingParams `json:"thinking_params"`
}

type saveChatModelsPayload struct {
	Models []chatModelPayload `json:"models" binding:"omitempty,dive"`
}

func chatModelsResponse(c *gin.Context) {
	models, err := llm.ChatModels()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"default_model": settings.OpenAISettings.Model,
		"models":        models,
	})
}

// GetChatModels lists the models offered in the assistant and the default.
func GetChatModels(c *gin.Context) {
	chatModelsResponse(c)
}

// SaveChatModels replaces the models offered in the assistant.
func SaveChatModels(c *gin.Context) {
	var payload saveChatModelsPayload
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	models := make([]model.LLMModel, 0, len(payload.Models))
	for _, m := range payload.Models {
		models = append(models, model.LLMModel{
			Name:           m.Name,
			ThinkingPreset: m.ThinkingPreset,
			ThinkingParams: m.ThinkingParams,
		})
	}

	if err := llm.SaveChatModels(models); err != nil {
		var cErr *cosy.Error
		if errors.As(err, &cErr) {
			c.AbortWithStatusJSON(http.StatusBadRequest, cErr)
			return
		}
		cosy.ErrHandler(c, err)
		return
	}

	chatModelsResponse(c)
}
