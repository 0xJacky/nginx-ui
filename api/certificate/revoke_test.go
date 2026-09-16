package certificate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevokeCertKeepsRecordWhenRevocationFails(t *testing.T) {
	db := setupCertTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AcmeUser{}))
	query.SetDefault(db)

	// No ACME account exists, so the revocation fails before reaching a CA.
	record := &model.Cert{Name: "example.com", Domains: []string{"example.com"}, ACMEUserID: 42}
	require.NoError(t, db.Create(record).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/certs/:id/revoke", RevokeCert)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	header := http.Header{}
	header.Set("Origin", srv.URL)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/certs/" + strconv.FormatUint(record.ID, 10) + "/revoke"
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	require.NoError(t, err)
	defer conn.Close()

	var statuses []string
	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.Status != "" {
			statuses = append(statuses, msg.Status)
		}
	}

	assert.Contains(t, statuses, Error)
	assert.NotContains(t, statuses, Success, "a failed revocation must not be reported as successful")

	var count int64
	require.NoError(t, db.Model(&model.Cert{}).Where("id = ?", record.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the certificate record must survive a failed revocation")
}
