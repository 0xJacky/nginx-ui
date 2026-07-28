package cluster

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPairingCodeIsSingleUseAndStoresOnlyControllerPublicKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.NodePairingCode{}, &model.NodeControllerCredential{}))
	model.Use(database)
	originalInstanceID := settings.NodeSettings.InstanceID
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		model.Use(nil)
	})

	router := gin.New()
	router.POST("/pairing-codes", func(c *gin.Context) {
		c.Set("user", &model.User{Model: model.Model{ID: 42}})
		CreatePairingCode(c)
	})
	router.POST("/pair/complete", CompletePairing)

	codeRecorder := httptest.NewRecorder()
	router.ServeHTTP(codeRecorder, httptest.NewRequest(http.MethodPost, "/pairing-codes", nil))
	require.Equal(t, http.StatusCreated, codeRecorder.Code)
	var codeResponse pairingCodeResponse
	require.NoError(t, json.Unmarshal(codeRecorder.Body.Bytes(), &codeResponse))
	decodedCode, err := base64.RawURLEncoding.DecodeString(codeResponse.Code)
	require.NoError(t, err)
	assert.Len(t, decodedCode, 16)
	assert.WithinDuration(t, time.Now().Add(pairingCodeLifetime), codeResponse.ExpiresAt, 2*time.Second)

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	payload, err := json.Marshal(completePairingRequest{
		Code:                 codeResponse.Code,
		ControllerInstanceID: "22222222-2222-4222-8222-222222222222",
		PublicKey:            base64.RawURLEncoding.EncodeToString(publicKey),
	})
	require.NoError(t, err)

	completeRecorder := httptest.NewRecorder()
	router.ServeHTTP(completeRecorder, httptest.NewRequest(http.MethodPost, "/pair/complete", bytes.NewReader(payload)))
	require.Equal(t, http.StatusCreated, completeRecorder.Code)
	var completeResponse completePairingResponse
	require.NoError(t, json.Unmarshal(completeRecorder.Body.Bytes(), &completeResponse))
	assert.Equal(t, settings.NodeSettings.InstanceID, completeResponse.TargetInstanceID)

	var credential model.NodeControllerCredential
	require.NoError(t, database.Where("credential_id = ?", completeResponse.CredentialID).First(&credential).Error)
	assert.Equal(t, []byte(publicKey), []byte(credential.PublicKey))
	assert.Empty(t, credential.PendingPublicKey)
	assert.Empty(t, credential.PreviousPublicKey)
	assert.Equal(t, model.NodeCredentialStatusActive, credential.Status)

	replayRecorder := httptest.NewRecorder()
	router.ServeHTTP(replayRecorder, httptest.NewRequest(http.MethodPost, "/pair/complete", bytes.NewReader(payload)))
	assert.Equal(t, http.StatusForbidden, replayRecorder.Code)
}

func TestCompletePairingRejectsExpiredCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.NodePairingCode{}, &model.NodeControllerCredential{}))
	model.Use(database)
	originalInstanceID := settings.NodeSettings.InstanceID
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		model.Use(nil)
	})

	code := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 16))
	codeHash := sha256.Sum256([]byte(code))
	require.NoError(t, database.Create(&model.NodePairingCode{
		CodeHash:  codeHash[:],
		ExpiresAt: time.Now().Add(-time.Second),
		CreatedBy: 1,
	}).Error)
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	payload, err := json.Marshal(completePairingRequest{
		Code:                 code,
		ControllerInstanceID: "22222222-2222-4222-8222-222222222222",
		PublicKey:            base64.RawURLEncoding.EncodeToString(publicKey),
	})
	require.NoError(t, err)

	router := gin.New()
	router.POST("/pair/complete", CompletePairing)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/pair/complete", bytes.NewReader(payload)))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestPairingRequiresHTTPSExceptLoopback(t *testing.T) {
	assert.NoError(t, requireSecurePairingURL("https://node.example"))
	assert.NoError(t, requireSecurePairingURL("http://127.0.0.1:9000"))
	assert.NoError(t, requireSecurePairingURL("http://[::1]:9000"))
	assert.ErrorContains(t, requireSecurePairingURL("http://node.example"), "HTTPS")
}
