package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetCurrentVersionIdentifiesTheRunningProcessWithoutCaching(t *testing.T) {
	var firstInstance string
	for range 2 {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		GetCurrentVersion(c)

		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		var result struct {
			version.Info
			InstanceID string `json:"instance_id"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Equal(t, *version.GetVersionInfo(), result.Info)
		_, err := uuid.Parse(result.InstanceID)
		require.NoError(t, err)
		if firstInstance == "" {
			firstInstance = result.InstanceID
		}
		require.Equal(t, firstInstance, result.InstanceID)
	}
}
