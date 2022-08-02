package model

import (
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"time"
)

type Auth struct {
	Model

	Name     string `json:"name"`
	Password string `json:"-"`
}

type AuthToken struct {
	Token string `json:"token"`
}

type JWTClaims struct {
	Name string `json:"name"`
	jwt.StandardClaims
}

func GetUser(name string) (user Auth, err error) {
	err = db.Where("name = ?", name).First(&user).Error
	if err != nil {
		return Auth{}, err
	}
	return user, err
}

func GetUserList(c *gin.Context, username interface{}) (data DataList) {
	var total int64
	db.Model(&Auth{}).Count(&total)
	var users []Auth

	result := db.Model(&Auth{}).Scopes(orderAndPaginate(c))

	if username != "" {
		result = result.Where("name LIKE ?", "%"+username.(string)+"%")
	}

	result.Find(&users)

	data = GetListWithPagination(&users, c, total)
	return
}

func DeleteToken(token string) error {
	return db.Where("token = ?", token).Delete(&AuthToken{}).Error
}

func CheckToken(token string) int64 {
	return db.Where("token = ?", token).Find(&AuthToken{}).RowsAffected
}

func GenerateJWT(name string) (string, error) {
	claims := JWTClaims{
		Name: name,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	}
	unsignedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := unsignedToken.SignedString([]byte(settings.ServerSettings.JwtSecret))
	if err != nil {
		return "", err
	}

	err = db.Create(&AuthToken{
		Token: signedToken,
	}).Error

	if err != nil {
		return "", err
	}

	return signedToken, err
}
