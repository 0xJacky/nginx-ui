package external_notify

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

const pluginNotifierType = "plugin:testchat"

// pluginSource serves pluginNotifierType and rejects a config without url.
type pluginSource struct {
	mu        sync.Mutex
	validated int
}

func (s *pluginSource) Channels() []notification.ExternalNotifierChannel {
	return []notification.ExternalNotifierChannel{{
		Type:     pluginNotifierType,
		Name:     "Test Chat",
		PluginID: "io.github.example.chat",
		Fields: []notification.ExternalNotifierField{
			{Key: "url", DisplayName: "URL", Required: true},
			{Key: "token", DisplayName: "Token", Secret: true},
		},
	}}
}

func (s *pluginSource) Handler(string) (notification.ExternalNotifierHandlerFunc, bool) {
	return nil, false
}

func (s *pluginSource) Validate(_ context.Context, notifierType string, config map[string]string) error {
	if notifierType != pluginNotifierType {
		return nil
	}
	s.mu.Lock()
	s.validated++
	s.mu.Unlock()
	if config["url"] == "" {
		return cosy.WrapErrorWithParams(notification.ErrInvalidNotifierField, "url", "url is required")
	}
	return nil
}

func (s *pluginSource) validations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validated
}

var registerPluginSource = sync.OnceValue(func() *pluginSource {
	source := &pluginSource{}
	notification.RegisterExternalNotifierSource(source)
	return source
})

func TestListChannelsReturnsPluginChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registerPluginSource()

	recorder := externalNotifyRequest(newExternalNotifySecurityRouter(nil, nil, &model.User{Model: model.Model{ID: 1}}),
		http.MethodGet, "/external_notifies/channels", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var body struct {
		Data []notification.ExternalNotifierChannel `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	require.Equal(t, pluginNotifierType, body.Data[0].Type)
	require.True(t, body.Data[0].Fields[1].Secret)
}

func setupExternalNotifyDB(t *testing.T) {
	t.Helper()
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.User{}, model.ExternalNotify{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "external-notify.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)
}

func TestPluginNotifierConfigIsValidatedBeforeSaving(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := registerPluginSource()
	setupExternalNotifyDB(t)

	admin := &model.User{Model: model.Model{ID: 104}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	require.NoError(t, model.UseDB().Create(admin).Error)
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newExternalNotifySecurityRouter(nil, nil, admin)
	before := source.validations()

	// A config the plugin rejects is not stored.
	recorder := externalNotifyRequest(router, http.MethodPost, "/external_notifies",
		`{"type":"`+pluginNotifierType+`","language":"en","config":{"token":"x"}}`, session)
	require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"code":400004`)
	count, err := query.ExternalNotify.Count()
	require.NoError(t, err)
	require.Zero(t, count)

	recorder = externalNotifyRequest(router, http.MethodPost, "/external_notifies",
		`{"type":"`+pluginNotifierType+`","language":"en","config":{"url":"https://chat.example","token":"x"}}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created model.ExternalNotify
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, before+2, source.validations())

	// Toggling the switch leaves type and config alone and is not validated.
	path := "/external_notifies/" + strconv.FormatUint(created.ID, 10)
	recorder = externalNotifyRequest(router, http.MethodPost, path, `{"enabled":false}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, before+2, source.validations())

	// Changing the config is validated against the stored type.
	recorder = externalNotifyRequest(router, http.MethodPost, path, `{"config":{"token":"y"}}`, session)
	require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, before+3, source.validations())

	// A built-in type never reaches the plugin source.
	recorder = externalNotifyRequest(router, http.MethodPost, "/external_notifies",
		`{"type":"bark","language":"en","config":{"device_key":"k"}}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, before+3, source.validations())
}
