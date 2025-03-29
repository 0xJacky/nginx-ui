package user

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type RecoveryCodesResponse struct {
	Message string `json:"message"`
	model.RecoveryCodes
}

func generateRecoveryCode() string {
	// generate recovery code, 10 hex numbers with a dash in the middle
	return fmt.Sprintf("%05x-%05x", rand.Intn(0x100000), rand.Intn(0x100000))
}

func generateRecoveryCodes(count int) []*model.RecoveryCode {
	recoveryCodes := make([]*model.RecoveryCode, count)
	for i := 0; i < count; i++ {
		recoveryCodes[i] = &model.RecoveryCode{
			Code: generateRecoveryCode(),
		}
	}
	return recoveryCodes
}

func ViewRecoveryCodes(c *gin.Context) {
	user := api.CurrentUser(c)

	// update last viewed time
	u := query.User
	t := time.Now().Unix()
	user.RecoveryCodes.LastViewed = &t
	_, err := u.Where(u.ID.Eq(user.ID)).Updates(user)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: user.RecoveryCodes,
	})
}

func GenerateRecoveryCodes(c *gin.Context) {
	user := api.CurrentUser(c)

	t := time.Now().Unix()
	recoveryCodes := model.RecoveryCodes{Codes: generateRecoveryCodes(16), LastViewed: &t}
	user.RecoveryCodes = recoveryCodes

	u := query.User
	_, err := u.Where(u.ID.Eq(user.ID)).Updates(user)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: recoveryCodes,
	})
}
