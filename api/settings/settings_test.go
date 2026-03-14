package settings

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSaveSettingsRejectsNegativeLogrotateInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings",
		bytes.NewBufferString(`{
			"auth":{"ban_threshold_minutes":1,"max_attempts":1},
			"cert":{"renewal_interval":7},
			"logrotate":{"enabled":true,"interval":-1}
		}`))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveSettings(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), appsettings.InvalidLogrotateIntervalMessage)
}
