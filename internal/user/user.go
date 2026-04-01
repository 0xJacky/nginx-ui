package user

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const ExpiredTime = 24 * time.Hour

type JWTClaims struct {
	Name   string `json:"name"`
	UserID uint64 `json:"user_id"`
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
	// Remove from cache first
	InvalidateTokenCache(token)

	// Remove from database
	q := query.AuthToken
	_, _ = q.Where(q.Token.Eq(token)).Delete()
}

func DeleteUserTokens(userID uint64) {
	if userID == 0 {
		return
	}

	InvalidateUserCache(userID)

	db := model.UseDB()
	if db == nil {
		return
	}

	var authTokens []model.AuthToken
	if err := db.Where("user_id = ?", userID).Find(&authTokens).Error; err != nil {
		logger.Error(err)
		return
	}

	for _, authToken := range authTokens {
		if authToken.Token != "" {
			InvalidateTokenCache(authToken.Token)
		}
	}

	if err := db.Where("user_id = ?", userID).Delete(&model.AuthToken{}).Error; err != nil {
		logger.Error(err)
	}
}

func getActiveUserByID(userID uint64) (*model.User, bool) {
	u := query.User
	user, err := u.FirstByID(userID)
	if err != nil {
		return nil, false
	}

	if !user.Status {
		DeleteUserTokens(user.ID)
		return nil, false
	}

	CacheUser(user)
	return user, true
}

func GetTokenUser(token string) (*model.User, bool) {
	_, err := ValidateJWT(token)
	if err != nil {
		logger.Error(err)
		return nil, false
	}

	// Try to get from cache first
	if tokenData, found := GetCachedTokenData(token); found {
		return getActiveUserByID(tokenData.UserID)
	}

	// Not in cache, load from database
	q := query.AuthToken
	authToken, err := q.Where(q.Token.Eq(token)).First()
	if err != nil {
		return nil, false
	}

	if authToken.ExpiredAt < time.Now().Unix() {
		DeleteToken(token)
		return nil, false
	}

	// Cache the token data
	CacheToken(authToken)

	return getActiveUserByID(authToken.UserID)
}

func GetTokenUserByShortToken(shortToken string) (*model.User, bool) {
	if shortToken == "" {
		return nil, false
	}

	// Try to get from cache first
	if tokenData, found := GetCachedShortTokenData(shortToken); found {
		return getActiveUserByID(tokenData.UserID)
	}

	// Not in cache, load from database
	db := model.UseDB()
	var authToken model.AuthToken
	err := db.Where("short_token = ?", shortToken).First(&authToken).Error
	if err != nil {
		return nil, false
	}

	if authToken.ExpiredAt < time.Now().Unix() {
		DeleteToken(authToken.Token)
		return nil, false
	}

	// Cache the token data
	CacheToken(&authToken)

	return getActiveUserByID(authToken.UserID)
}

type AccessTokenPayload struct {
	Token      string `json:"token,omitempty"`
	ShortToken string `json:"short_token,omitempty"`
}

func GenerateJWT(user *model.User) (*AccessTokenPayload, error) {
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
		return nil, err
	}

	// Generate 16-byte short token (16 characters)
	shortTokenBytes := make([]byte, 16)
	_, err = rand.Read(shortTokenBytes)
	if err != nil {
		return nil, err
	}
	// Use base64 URL encoding to get a 16-character string
	shortToken := base64.URLEncoding.EncodeToString(shortTokenBytes)[:16]

	authToken := &model.AuthToken{
		UserID:     user.ID,
		Token:      signedToken,
		ShortToken: shortToken,
		ExpiredAt:  now.Add(ExpiredTime).Unix(),
	}

	q := query.AuthToken
	err = q.Create(authToken)

	if err != nil {
		return nil, err
	}

	// Cache the new token
	CacheToken(authToken)

	return &AccessTokenPayload{
		Token: signedToken,
	}, nil
}

// GenerateShortToken creates a standalone short token for WebSocket authentication.
// The short token is stored in a new AuthToken row with no associated JWT.
func GenerateShortToken(userID uint64) (string, error) {
	shortTokenBytes := make([]byte, 16)
	_, err := rand.Read(shortTokenBytes)
	if err != nil {
		return "", err
	}
	shortToken := base64.URLEncoding.EncodeToString(shortTokenBytes)[:16]

	now := time.Now()
	authToken := &model.AuthToken{
		UserID:     userID,
		ShortToken: shortToken,
		ExpiredAt:  now.Add(ExpiredTime).Unix(),
	}

	q := query.AuthToken
	err = q.Create(authToken)
	if err != nil {
		return "", err
	}

	CacheToken(authToken)
	return shortToken, nil
}

func ValidateJWT(tokenStr string) (claims *JWTClaims, err error) {
	if tokenStr == "" {
		err = ErrTokenIsEmpty
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
	return nil, ErrInvalidClaimsType
}
