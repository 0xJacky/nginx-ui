package user

import (
	cryptorand "crypto/rand"
	"math/big"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

const recoveryCodeAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

type RecoveryCodesResponse struct {
	Message string `json:"message"`
	model.RecoveryCodes
}

func randomRecoveryCodeChar() (byte, error) {
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(recoveryCodeAlphabet))))
	if err != nil {
		return 0, err
	}

	return recoveryCodeAlphabet[n.Int64()], nil
}

func generateRecoveryCode() (string, error) {
	code := make([]byte, 11)
	for i := range code {
		if i == 5 {
			code[i] = '-'
			continue
		}

		char, err := randomRecoveryCodeChar()
		if err != nil {
			return "", err
		}
		code[i] = char
	}

	return string(code), nil
}

func generateRecoveryCodes(count int) ([]*model.RecoveryCode, error) {
	recoveryCodes := make([]*model.RecoveryCode, count)
	for i := 0; i < count; i++ {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}

		recoveryCodes[i] = &model.RecoveryCode{
			Code: code,
		}
	}
	return recoveryCodes, nil
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
	codes, err := generateRecoveryCodes(16)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	recoveryCodes := model.RecoveryCodes{Codes: codes, LastViewed: &t}
	user.RecoveryCodes = recoveryCodes

	u := query.User
	_, err = u.Where(u.ID.Eq(user.ID)).Updates(user)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, RecoveryCodesResponse{
		Message:       "ok",
		RecoveryCodes: recoveryCodes,
	})
}
