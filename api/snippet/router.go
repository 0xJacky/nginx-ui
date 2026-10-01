package snippet

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers the snippet endpoints.
func InitRouter(r *gin.RouterGroup) {
	r.GET("snippets", GetSnippets)
	r.GET("snippets/:file", GetSnippet)
	r.GET("snippet_sync", GetSnippetSync)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("snippets", CreateSnippet)
		o.POST("snippets/:file", ModifySnippet)
		o.DELETE("snippets/:file", DeleteSnippet)
		o.POST("snippet_sync", SaveSnippetSync)
	}
}
