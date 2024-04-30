package cosy

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm"
	"net/http"
)

func (c *Ctx[T]) UpdateOrder() {
	var json struct {
		TargetID    int   `json:"target_id"`
		Direction   int   `json:"direction" binding:"oneof=-1 1"`
		AffectedIDs []int `json:"affected_ids"`
	}

	if !api.BindAndValid(c.ctx, &json) {
		return
	}

	affectedLen := len(json.AffectedIDs)

	db := model.UseDB()

	if c.table != "" {
		db = db.Table(c.table, c.tableArgs...)
	}

	// update target
	err := db.Model(&c.Model).Where("id = ?", json.TargetID).Update("order_id", gorm.Expr("order_id + ?", affectedLen*(-json.Direction))).Error

	if err != nil {
		api.ErrHandler(c.ctx, err)
		return
	}

	// update affected
	err = db.Model(&c.Model).Where("id in ?", json.AffectedIDs).Update("order_id", gorm.Expr("order_id + ?", json.Direction)).Error

	if err != nil {
		api.ErrHandler(c.ctx, err)
		return
	}

	c.ctx.JSON(http.StatusOK, json)
}
