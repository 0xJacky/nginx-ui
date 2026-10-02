package llm

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func useChatModelDB(t *testing.T, defaultModel string) {
	t.Helper()

	originalDB := model.UseDB()
	originalQuery := query.LLMModel
	originalSettings := settings.OpenAISettings

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.LLMModel{}))

	model.Use(db)
	testQuery := query.Use(db)
	query.LLMModel = &testQuery.LLMModel
	settings.OpenAISettings = &settings.OpenAI{Model: defaultModel}

	t.Cleanup(func() {
		model.Use(originalDB)
		query.LLMModel = originalQuery
		settings.OpenAISettings = originalSettings
	})
}

func requireCosyCode(t *testing.T, err error, want error) {
	t.Helper()
	var got, expected *cosy.Error
	require.ErrorAs(t, err, &got)
	require.ErrorAs(t, want, &expected)
	assert.Equal(t, expected.Code, got.Code)
}

func TestChatModelsFallBackToTheDefaultModel(t *testing.T) {
	useChatModelDB(t, "gpt-5")

	models, err := ChatModels()

	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-5", models[0].Name)
}

func TestSaveChatModelsReplacesTheListInOrder(t *testing.T) {
	useChatModelDB(t, "gpt-5")

	require.NoError(t, SaveChatModels([]model.LLMModel{{Name: "a"}, {Name: "b"}}))
	require.NoError(t, SaveChatModels([]model.LLMModel{
		{Name: " deepseek-flash ", ThinkingPreset: "thinking_type", ThinkingParams: model.LLMThinkingParams{
			ThinkingOff:  {"thinking": map[string]any{"type": "disabled"}},
			ThinkingHigh: {"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "high"},
		}},
		{Name: "gpt-5"},
	}))

	models, err := ChatModels()
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "deepseek-flash", models[0].Name)
	assert.Equal(t, "gpt-5", models[1].Name)
	assert.Equal(t, "none", models[1].ThinkingPreset)
	assert.Equal(t, "high", models[0].ThinkingParams[ThinkingHigh]["reasoning_effort"])
}

func TestSaveChatModelsRejectsInvalidLists(t *testing.T) {
	useChatModelDB(t, "gpt-5")

	requireCosyCode(t, SaveChatModels([]model.LLMModel{{Name: "a"}, {Name: "a"}}), ErrDuplicateChatModel)
	requireCosyCode(t, SaveChatModels([]model.LLMModel{{Name: "a", ThinkingPreset: "magic"}}), ErrUnknownThinkingPreset)
	requireCosyCode(t, SaveChatModels([]model.LLMModel{{
		Name:           "a",
		ThinkingParams: model.LLMThinkingParams{"ultra": {}},
	}}), ErrUnknownThinkingLevel)
}

func TestChatRequestOptions(t *testing.T) {
	useChatModelDB(t, "gpt-5")
	require.NoError(t, SaveChatModels([]model.LLMModel{
		{Name: "gpt-5", ThinkingPreset: "openai", ThinkingParams: model.LLMThinkingParams{
			ThinkingLow: {"reasoning_effort": "low"},
		}},
		{Name: "qwen3:32b"},
	}))

	name, extra, err := ChatRequestOptions("", ThinkingLow)
	require.NoError(t, err)
	assert.Equal(t, "gpt-5", name)
	assert.Equal(t, map[string]any{"reasoning_effort": "low"}, extra)

	name, extra, err = ChatRequestOptions("qwen3:32b", "")
	require.NoError(t, err)
	assert.Equal(t, "qwen3:32b", name)
	assert.Nil(t, extra)

	_, _, err = ChatRequestOptions("qwen3:32b", ThinkingHigh)
	requireCosyCode(t, err, ErrThinkingLevelNotConfigured)

	_, _, err = ChatRequestOptions("not-enabled", "")
	requireCosyCode(t, err, ErrChatModelNotEnabled)
}

func TestChatRequestOptionsKeepsTheDefaultModelUsable(t *testing.T) {
	useChatModelDB(t, "gpt-5")
	require.NoError(t, SaveChatModels([]model.LLMModel{{Name: "qwen3:32b"}}))

	name, extra, err := ChatRequestOptions("", "")

	require.NoError(t, err)
	assert.Equal(t, "gpt-5", name)
	assert.Nil(t, extra)
}
