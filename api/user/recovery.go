package user

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
)

type RecoveryCodesResponse struct {
	Message string `json:"message"`
	model.RecoveryCodes
}

func generateRecoveryCode() string {
	// generate recovery code, 10 hex numbers with a dash in the middle
	return fmt.Sprintf("%05x-%05x", rand.Intn(0x100000), rand.Intn(0x100000))
}

func generateRecoveryCodes(count int) []model.RecoveryCode {
	recoveryCodes := make([]model.RecoveryCode, count)
	for i := 0; i < count; i++ {
		recoveryCodes[i].Code = generateRecoveryCode()
	}
	return recoveryCodes
}

func ViewRecoveryCodes(c *gin.Context) {
	user := api.CurrentUser(c)

	u := query.User
	user, err := u.Where(u.ID.Eq(user.ID)).First()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	// update last viewed time
	t := time.Now()
	user.RecoveryCodes.LastViewed = &t
	_, err = u.Where(u.ID.Eq(user.ID)).Updates(user)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: user.RecoveryCodes,
	})
}

func GenerateRecoveryCodes(c *gin.Context) {
	user := api.CurrentUser(c)

	t := time.Now()
	recoveryCodes := model.RecoveryCodes{Codes: generateRecoveryCodes(16), LastViewed: &t}
	codesJson, err := json.Marshal(&recoveryCodes)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	u := query.User
	_, err = u.Where(u.ID.Eq(user.ID)).Update(u.RecoveryCodes, codesJson)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: recoveryCodes,
	})
}
