package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

func TestSettingsSaveErrorReportsUnstorableValueAsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	SettingsSaveError(c, cosy.WrapErrorWithParams(settings.ErrUnstorableValue, "plugin.MarketplaceSources"))

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response cosy.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "settings", response.Scope)
	assert.Equal(t, int32(40007), response.Code)
	assert.Equal(t, []string{"plugin.MarketplaceSources"}, response.Params)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	SettingsSaveError(c, errors.New("disk full"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
