package user

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/dgrijalva/jwt-go"
	"time"
)

type JWTClaims struct {
	Name string `json:"name"`
	jwt.StandardClaims
}

func GetUser(name string) (user model.Auth, err error) {
	db := model.UseDB()
	err = db.Where("name", name).First(&user).Error
	if err != nil {
		return
	}
	return
}

func DeleteToken(token string) error {
	db := model.UseDB()
	return db.Where("token", token).Delete(&model.AuthToken{}).Error
}

func CheckToken(token string) int64 {
	db := model.UseDB()
	return db.Where("token", token).Find(&model.AuthToken{}).RowsAffected
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

	db := model.UseDB()
	err = db.Create(&model.AuthToken{
		Token: signedToken,
	}).Error

	if err != nil {
		return "", err
	}

	return signedToken, err
}
