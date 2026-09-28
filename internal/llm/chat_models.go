package llm

import (
	"slices"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

// Thinking levels a chat can ask for. Each model maps the levels it supports
// to the request fields its provider expects.
const (
	ThinkingOff    = "off"
	ThinkingLow    = "low"
	ThinkingMedium = "medium"
	ThinkingHigh   = "high"
)

var ThinkingLevels = []string{ThinkingOff, ThinkingLow, ThinkingMedium, ThinkingHigh}

// Thinking presets name the provider convention the settings page filled the
// thinking params from. The server only stores the name for the page to show.
var ThinkingPresets = []string{
	"none",
	"openai",
	"thinking_type",
	"qwen",
	"openrouter",
	"anthropic",
	"chat_template",
	"custom",
}

// ChatModels returns the models offered in the assistant, in their saved
// order. With none saved, the default model is offered on its own so that
// configurations from before the model list keep working.
func ChatModels() ([]*model.LLMModel, error) {
	q := query.LLMModel
	models, err := q.Order(q.SortOrder, q.ID).Find()
	if err != nil {
		return nil, err
	}

	if len(models) == 0 && settings.OpenAISettings.Model != "" {
		models = []*model.LLMModel{{Name: settings.OpenAISettings.Model, ThinkingPreset: "none"}}
	}

	return models, nil
}

// SaveChatModels replaces the models offered in the assistant.
func SaveChatModels(models []model.LLMModel) error {
	seen := make(map[string]struct{}, len(models))
	for i := range models {
		models[i].Name = strings.TrimSpace(models[i].Name)
		if _, ok := seen[models[i].Name]; ok {
			return cosy.WrapErrorWithParams(ErrDuplicateChatModel, models[i].Name)
		}
		seen[models[i].Name] = struct{}{}

		if models[i].ThinkingPreset == "" {
			models[i].ThinkingPreset = "none"
		}
		if !slices.Contains(ThinkingPresets, models[i].ThinkingPreset) {
			return cosy.WrapErrorWithParams(ErrUnknownThinkingPreset, models[i].ThinkingPreset)
		}
		for level := range models[i].ThinkingParams {
			if !slices.Contains(ThinkingLevels, level) {
				return cosy.WrapErrorWithParams(ErrUnknownThinkingLevel, level)
			}
		}

		models[i].ID = 0
		models[i].SortOrder = i
	}

	return model.UseDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.LLMModel{}).Error; err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}
		return tx.Create(&models).Error
	})
}

// ChatRequestOptions resolves the model and extra request body for a chat.
// An empty model name means the default model, an empty level leaves the
// provider's own default in place.
func ChatRequestOptions(modelName, level string) (string, map[string]any, error) {
	if modelName == "" {
		modelName = settings.OpenAISettings.Model
	}

	models, err := ChatModels()
	if err != nil {
		return "", nil, err
	}

	index := slices.IndexFunc(models, func(m *model.LLMModel) bool {
		return m.Name == modelName
	})
	if index < 0 {
		// The default model stays usable even when it was left out of the list.
		if modelName == settings.OpenAISettings.Model && level == "" {
			return modelName, nil, nil
		}
		return "", nil, cosy.WrapErrorWithParams(ErrChatModelNotEnabled, modelName)
	}

	if level == "" {
		return modelName, nil, nil
	}

	extra, ok := models[index].ThinkingParams[level]
	if !ok {
		return "", nil, cosy.WrapErrorWithParams(ErrThinkingLevelNotConfigured, level, modelName)
	}

	return modelName, extra, nil
}
