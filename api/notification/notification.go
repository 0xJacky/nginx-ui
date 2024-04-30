package notification

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cosy"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
)

func Get(c *gin.Context) {
	n := query.Notification

	id := cast.ToInt(c.Param("id"))

	data, err := n.FirstByID(id)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, data)
}

func GetList(c *gin.Context) {
	cosy.Core[model.Notification](c).PagingList()
}

func Destroy(c *gin.Context) {
	cosy.Core[model.Notification](c).
		PermanentlyDelete()
}

func DestroyAll(c *gin.Context) {
	db := model.UseDB()
	// remove all records
	err := db.Exec("DELETE FROM notifications").Error

	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	// reset auto increment
	err = db.Exec("UPDATE sqlite_sequence SET seq = 0 WHERE name = 'notifications';").Error

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
