package openai

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// ChatGPT
	r.POST("chatgpt", MakeChatCompletionRequest)
	r.POST("chatgpt_record", StoreChatGPTRecord)
}
