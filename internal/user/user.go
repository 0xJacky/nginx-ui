package user

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	"strings"
	"time"
)

const ExpiredTime = 24 * time.Hour

type JWTClaims struct {
	Name   string `json:"name"`
	UserID int    `json:"user_id"`
	jwt.StandardClaims
}

func BuildCacheTokenKey(token string) string {
	var sb strings.Builder
	sb.WriteString("token:")
	sb.WriteString(token)
	return sb.String()
}

func GetUser(name string) (user *model.Auth, err error) {
	db := model.UseDB()
	user = &model.Auth{}
	err = db.Where("name", name).First(user).Error
	if err != nil {
		return
	}
	return
}

func DeleteToken(token string) {
	q := query.AuthToken
	_, _ = q.Where(q.Token.Eq(token)).Delete()
}

func GetTokenUser(token string) (*model.Auth, bool) {
	q := query.AuthToken
	authToken, err := q.Where(q.Token.Eq(token)).First()
	if err != nil {
		return nil, false
	}

	if authToken.ExpiredAt < time.Now().Unix() {
		DeleteToken(token)
		return nil, false
	}

	u := query.Auth
	user, err := u.FirstByID(authToken.UserID)
	return user, err == nil
}

func GenerateJWT(user *model.Auth) (string, error) {
	claims := JWTClaims{
		Name:   user.Name,
		UserID: user.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(ExpiredTime).Unix(),
		},
	}

	unsignedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := unsignedToken.SignedString([]byte(settings.ServerSettings.JwtSecret))
	if err != nil {
		return "", err
	}

	q := query.AuthToken
	err = q.Create(&model.AuthToken{
		UserID:    user.ID,
		Token:     signedToken,
		ExpiredAt: time.Now().Add(ExpiredTime).Unix(),
	})

	if err != nil {
		return "", err
	}

	return signedToken, err
}

func ValidateJWT(token string) (claims *JWTClaims, err error) {
	if token == "" {
		err = errors.New("token is empty")
		return
	}
	unsignedToken, err := jwt.ParseWithClaims(
		token,
		&JWTClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(settings.ServerSettings.JwtSecret), nil
		},
	)
	if err != nil {
		err = errors.New("parse with claims error")
		return
	}
	claims, ok := unsignedToken.Claims.(*JWTClaims)
	if !ok {
		err = errors.New("convert to jwt claims error")
		return
	}
	if claims.ExpiresAt < time.Now().UTC().Unix() {
		err = errors.New("jwt is expired")
	}
	return
}

func CurrentUser(token string) (u *model.Auth, err error) {
	// validate token
	var claims *JWTClaims
	claims, err = ValidateJWT(token)
	if err != nil {
		return
	}

	// get user by id
	user := query.Auth
	u, err = user.FirstByID(claims.UserID)
	if err != nil {
		return
	}

	logger.Info("[Current User]", u.Name)

	return
}
