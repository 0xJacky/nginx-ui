package llm

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("llm_messages", GetLLMRecord)
	r.POST("llm_messages", StoreLLMRecord)
}

// InitLocalRouter for main node only (no proxy)
func InitLocalRouter(r *gin.RouterGroup) {
	// LLM endpoints that should only run on main node
	r.POST("llm", MakeChatCompletionRequest)
	// Code Completion
	r.GET("code_completion", CodeCompletion)
	r.GET("code_completion/enabled", GetCodeCompletionEnabledStatus)
}
