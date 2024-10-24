package sites

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func GetCategory(c *gin.Context) {
	cosy.Core[model.SiteCategory](c).Get()
}

func GetCategoryList(c *gin.Context) {
	cosy.Core[model.SiteCategory](c).PagingList()
}

func AddCategory(c *gin.Context) {
	cosy.Core[model.SiteCategory](c).
		SetValidRules(gin.H{
			"name":          "required",
			"sync_node_ids": "omitempty",
		}).
		Create()
}

func ModifyCategory(c *gin.Context) {
	cosy.Core[model.SiteCategory](c).
		SetValidRules(gin.H{
			"name":          "required",
			"sync_node_ids": "omitempty",
		}).
		Modify()
}

func DeleteCategory(c *gin.Context) {
	cosy.Core[model.SiteCategory](c).Destroy()
}

func RecoverCategory(c *gin.Context) {
	cosy.Core[model.SiteCategory](c).Recover()
}
