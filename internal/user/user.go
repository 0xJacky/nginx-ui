package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/golang-jwt/jwt/v5"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if err := RevokeSessionToken(token); err != nil {
		logger.Error(err)
	}
}

func sessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RevokeSessionToken removes a JWT and only the short tokens bound to that login.
func RevokeSessionToken(token string) error {
	if token == "" {
		return nil
	}
	db := model.UseDB()
	if db == nil {
		return errors.New("database is not initialized")
	}

	hash := sessionTokenHash(token)
	var shortTokens []string
	err := db.Transaction(func(tx *gorm.DB) error {
		var parent model.AuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token = ?", token).Take(&parent).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		match := tx.Where("token = ? OR (session_hash = ? AND token = ?)", token, hash, "")
		if err := match.Model(&model.AuthToken{}).Pluck("short_token", &shortTokens).Error; err != nil {
			return err
		}
		return tx.Where("token = ? OR (session_hash = ? AND token = ?)", token, hash, "").Delete(&model.AuthToken{}).Error
	})
	if err != nil {
		return err
	}

	InvalidateTokenCache(token)
	for _, shortToken := range shortTokens {
		InvalidateShortTokenCache(shortToken)
	}
	return nil
}

// RevokeShortToken removes one WebSocket credential. A login-paired short
// token revokes its JWT session as well.
func RevokeShortToken(shortToken string) error {
	if shortToken == "" {
		return nil
	}
	db := model.UseDB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	var authToken model.AuthToken
	if err := db.Where("short_token = ?", shortToken).Take(&authToken).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if authToken.Token != "" {
		return RevokeSessionToken(authToken.Token)
	}
	if err := db.Where("short_token = ?", shortToken).Delete(&model.AuthToken{}).Error; err != nil {
		return err
	}
	InvalidateShortTokenCache(shortToken)
	return nil
}

func DeleteShortToken(shortToken string) {
	if err := RevokeShortToken(shortToken); err != nil {
		logger.Error(err)
	}
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
		if authToken.ShortToken != "" {
			InvalidateShortTokenCache(authToken.ShortToken)
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

	// Check the database even if this process has a cached token: logout can
	// revoke a session through another application instance.
	db := model.UseDB()
	if db == nil {
		return nil, false
	}
	var authToken model.AuthToken
	if err := db.Where("token = ?", token).Take(&authToken).Error; err != nil {
		return nil, false
	}

	if authToken.ExpiredAt < time.Now().Unix() {
		DeleteToken(token)
		return nil, false
	}

	return getActiveUserByID(authToken.UserID)
}

func GetTokenUserByShortToken(shortToken string) (*model.User, bool) {
	if shortToken == "" {
		return nil, false
	}

	// A cached short token is not sufficient: another application instance may
	// have deleted its row during logout or selective revocation.
	db := model.UseDB()
	if db == nil {
		return nil, false
	}
	var authToken model.AuthToken
	err := db.Where("short_token = ?", shortToken).Take(&authToken).Error
	if err != nil {
		return nil, false
	}

	if authToken.ExpiredAt < time.Now().Unix() {
		if authToken.Token != "" {
			DeleteToken(authToken.Token)
		} else {
			DeleteShortToken(authToken.ShortToken)
		}
		return nil, false
	}
	if authToken.Token == "" && authToken.SessionHash == "" {
		DeleteShortToken(shortToken)
		return nil, false
	}
	if !shortTokenSessionActive(authToken.Token, authToken.SessionHash) {
		InvalidateShortTokenCache(shortToken)
		return nil, false
	}

	return getActiveUserByID(authToken.UserID)
}

func shortTokenSessionActive(token, sessionHash string) bool {
	if token == "" && sessionHash == "" {
		return false
	}
	db := model.UseDB()
	if db == nil {
		return false
	}
	var count int64
	lookup := db.Model(&model.AuthToken{}).Where("expired_at >= ?", time.Now().Unix())
	if token != "" {
		lookup = lookup.Where("token = ?", token)
	} else {
		lookup = lookup.Where("session_hash = ? AND token <> ?", sessionHash, "")
	}
	if err := lookup.Count(&count).Error; err != nil {
		logger.Error(err)
		return false
	}
	return count > 0
}

type AccessTokenPayload struct {
	Token      string `json:"token,omitempty"`
	ShortToken string `json:"short_token,omitempty"`
}

type LoginProof string

const (
	LoginProofPassword LoginProof = "password"
	LoginProofOTP      LoginProof = "otp"
	LoginProofPasskey  LoginProof = "passkey"
	LoginProofExternal LoginProof = "external"
	LoginProofSystem   LoginProof = "system"
)

var ErrPasskeyRequired = errors.New("passkey verification is required")

// IssueLoginToken is the policy gate for every interactive login path.
func IssueLoginToken(user *model.User, proof LoginProof) (*AccessTokenPayload, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	if proof == LoginProofPassword && !user.EnabledOTP() {
		enabledPasskey, err := user.EnabledPasskey()
		if err != nil {
			return nil, err
		}
		if enabledPasskey {
			return nil, ErrPasskeyRequired
		}
	}
	return generateJWT(user)
}

// GenerateJWT remains available for non-login system flows and tests. New
// interactive login paths must call IssueLoginToken with an explicit proof.
func GenerateJWT(user *model.User) (*AccessTokenPayload, error) {
	return IssueLoginToken(user, LoginProofSystem)
}

func generateJWT(user *model.User) (*AccessTokenPayload, error) {
	sessionID := make([]byte, 16)
	if _, err := rand.Read(sessionID); err != nil {
		return nil, err
	}
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
			ID:        hex.EncodeToString(sessionID),
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
		UserID:      user.ID,
		Token:       signedToken,
		ShortToken:  shortToken,
		SessionHash: sessionTokenHash(signedToken),
		ExpiredAt:   now.Add(ExpiredTime).Unix(),
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

// GenerateShortTokenForSession binds a WebSocket token to the issuing JWT.
func GenerateShortTokenForSession(userID uint64, token string) (string, error) {
	claims, err := ValidateJWT(token)
	if err != nil || claims.UserID != userID {
		return "", ErrInvalidClaimsType
	}

	shortTokenBytes := make([]byte, 16)
	if _, err := rand.Read(shortTokenBytes); err != nil {
		return "", err
	}
	shortToken := base64.URLEncoding.EncodeToString(shortTokenBytes)[:16]
	hash := sessionTokenHash(token)
	now := time.Now().Unix()
	db := model.UseDB()
	if db == nil {
		return "", errors.New("database is not initialized")
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		var parent model.AuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token = ? AND user_id = ? AND expired_at >= ?", token, userID, now).
			Take(&parent).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.AuthToken{}).
			Where("token = ? AND user_id = ?", token, userID).
			Update("session_hash", hash).Error; err != nil {
			return err
		}
		return tx.Create(&model.AuthToken{
			UserID:      userID,
			ShortToken:  shortToken,
			SessionHash: hash,
			ExpiredAt:   parent.ExpiredAt,
		}).Error
	})
	if err != nil {
		return "", err
	}
	// The first WebSocket use loads the row. Avoid caching after a concurrent logout.
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
