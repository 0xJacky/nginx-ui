package openai

import "github.com/gin-gonic/gin"


func InitRouter(r *gin.RouterGroup) {
	// ChatGPT
	r.POST("chatgpt", MakeChatCompletionRequest)
	r.GET("chatgpt/history", GetChatGPTRecord)
	r.POST("chatgpt_record", StoreChatGPTRecord)
	// Code Completion
	r.GET("code_completion", CodeCompletion)
	r.GET("code_completion/enabled", GetCodeCompletionEnabledStatus)
}
