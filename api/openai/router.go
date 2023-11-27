package openai

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// ChatGPT
	r.POST("chat_gpt", MakeChatCompletionRequest)
	r.POST("chat_gpt_record", StoreChatGPTRecord)
}
