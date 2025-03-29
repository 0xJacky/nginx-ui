package notification

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func Get(c *gin.Context) {
	n := query.Notification

	id := cast.ToUint64(c.Param("id"))

	data, err := n.FirstByID(id)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, data)
}

func GetList(c *gin.Context) {
	cosy.Core[model.Notification](c).PagingList()
}

func Destroy(c *gin.Context) {
	cosy.Core[model.Notification](c).Destroy()
}

func DestroyAll(c *gin.Context) {
	db := model.UseDB()
	// remove all records
	err := db.Exec("DELETE FROM notifications").Error

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// reset auto increment
	err = db.Exec("UPDATE sqlite_sequence SET seq = 0 WHERE name = 'notifications';").Error

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
