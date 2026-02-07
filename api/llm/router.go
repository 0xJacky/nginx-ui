package llm

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// LLM Session endpoints
	r.GET("llm_sessions", GetLLMSessions)
	r.GET("llm_sessions/:session_id", GetLLMSession)
	r.POST("llm_sessions", CreateLLMSession)
	r.PUT("llm_sessions/:session_id", UpdateLLMSession)
	r.DELETE("llm_sessions/:session_id", DeleteLLMSession)
	r.POST("llm_sessions/:session_id/duplicate", DuplicateLLMSession)

	// Compatibility endpoints for legacy file-based sessions
	r.GET("llm_messages", GetLLMSessionByPath)
	r.POST("llm_messages", CreateOrUpdateLLMSessionByPath)
}

// InitLocalRouter for main node only (no proxy)
func InitLocalRouter(r *gin.RouterGroup) {
	// LLM endpoints that should only run on main node
	r.POST("llm", MakeChatCompletionRequest)
	// Code Completion
	r.GET("code_completion", CodeCompletion)
	r.GET("code_completion/enabled", GetCodeCompletionEnabledStatus)
	// Generate title from messages - uses local LLM config
	r.POST("generate_title", GenerateTitle)
}
