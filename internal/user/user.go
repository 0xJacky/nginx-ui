package user

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"time"
)

const ExpiredTime = 24 * time.Hour

type JWTClaims struct {
	Name   string `json:"name"`
	UserID int    `json:"user_id"`
	jwt.RegisteredClaims
}

func GetUser(name string) (user *model.User, err error) {
	db := model.UseDB()
	user = &model.User{}
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

func GetTokenUser(token string) (*model.User, bool) {
	_, err := ValidateJWT(token)
	if err != nil {
		logger.Error(err)
		return nil, false
	}

	q := query.AuthToken
	authToken, err := q.Where(q.Token.Eq(token)).First()
	if err != nil {
		return nil, false
	}

	if authToken.ExpiredAt < time.Now().Unix() {
		DeleteToken(token)
		return nil, false
	}

	u := query.User
	user, err := u.FirstByID(authToken.UserID)
	return user, err == nil
}

func GenerateJWT(user *model.User) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		Name:   user.Name,
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ExpiredTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "Nginx UI",
			Subject:   user.Name,
			ID:        cast.ToString(user.ID),
		},
	}

	unsignedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := unsignedToken.SignedString([]byte(cSettings.AppSettings.JwtSecret))
	if err != nil {
		return "", err
	}

	q := query.AuthToken
	err = q.Create(&model.AuthToken{
		UserID:    user.ID,
		Token:     signedToken,
		ExpiredAt: now.Add(ExpiredTime).Unix(),
	})

	if err != nil {
		return "", err
	}

	return signedToken, err
}

func ValidateJWT(tokenStr string) (claims *JWTClaims, err error) {
	if tokenStr == "" {
		err = errors.New("token is empty")
		return
	}
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cSettings.AppSettings.JwtSecret), nil
	})
	if err != nil {
		return
	}
	var ok bool
	if claims, ok = token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid claims type")
}
