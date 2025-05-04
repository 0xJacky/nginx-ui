package kernel

import (
	"context"

	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

func RegisterAcmeUser(ctx context.Context) {
	a := query.AcmeUser
	users, _ := a.Where(a.RegisterOnStartup.Is(true)).Find()
	for _, user := range users {
		err := user.Register()
		if err != nil {
			logger.Error(err)
		}
		_, err = a.Where(a.ID.Eq(user.ID)).Updates(user)
		if err != nil {
			logger.Error(err)
		}
	}
}
