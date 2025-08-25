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

func GetTokenUser(token string) (*model.User, bool) {
	_, err := ValidateJWT(token)
	if err != nil {
		logger.Error(err)
		return nil, false
	}

	// Try to get from cache first
	if tokenData, found := GetCachedTokenData(token); found {
		// Get user from cache or database
		if user, userFound := GetCachedUser(tokenData.UserID); userFound {
			return user, true
		}
		
		// User not in cache, load from database and cache it
		u := query.User
		user, err := u.FirstByID(tokenData.UserID)
		if err == nil {
			CacheUser(user)
			return user, true
		}
		return nil, false
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

	// Get user and cache it
	u := query.User
	user, err := u.FirstByID(authToken.UserID)
	if err == nil {
		CacheUser(user)
		return user, true
	}
	return user, err == nil
}

func GetTokenUserByShortToken(shortToken string) (*model.User, bool) {
	if shortToken == "" {
		return nil, false
	}

	// Try to get from cache first
	if tokenData, found := GetCachedShortTokenData(shortToken); found {
		// Get user from cache or database
		if user, userFound := GetCachedUser(tokenData.UserID); userFound {
			return user, true
		}
		
		// User not in cache, load from database and cache it
		u := query.User
		user, err := u.FirstByID(tokenData.UserID)
		if err == nil {
			CacheUser(user)
			return user, true
		}
		return nil, false
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

	// Get user and cache it
	u := query.User
	user, err := u.FirstByID(authToken.UserID)
	if err == nil {
		CacheUser(user)
		return user, true
	}
	return user, err == nil
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
		Token:      signedToken,
		ShortToken: shortToken,
	}, nil
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

