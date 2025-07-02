package kernel

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

func InitUser() {
	db := cosy.UseDB(context.Background())
	user := &model.User{}
	db.Unscoped().Where("id = ?", 1).Find(user)

	// if user is not found, create a new user
	if user.ID == 0 {
		db.Create(&model.User{
			Model: model.Model{
				ID: 1,
			},
			Name: "admin",
		})
		return 
	}

	// if user is found, check if the user is deleted
	// if the user is deleted, restore the user
	db.Unscoped().Where("id = ?", 1).Update("deleted_at", nil)
}
