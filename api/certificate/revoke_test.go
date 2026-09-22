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
	"gorm.io/gorm"
)

// revokeCertStatuses runs the revoke WebSocket handler for record with the
// given query string and returns the status of every message it sent.
func revokeCertStatuses(t *testing.T, record *model.Cert, rawQuery string) []string {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/certs/:id/revoke", RevokeCert)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	header := http.Header{}
	header.Set("Origin", srv.URL)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/certs/" + strconv.FormatUint(record.ID, 10) + "/revoke"
	if rawQuery != "" {
		url += "?" + rawQuery
	}
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

	return statuses
}

// createUnrevokableCert stores a certificate whose ACME account does not
// exist, so the revocation fails before reaching a CA.
func createUnrevokableCert(t *testing.T) (*gorm.DB, *model.Cert) {
	t.Helper()

	db := setupCertTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AcmeUser{}))
	query.SetDefault(db)

	record := &model.Cert{Name: "example.com", Domains: []string{"example.com"}, ACMEUserID: 42}
	require.NoError(t, db.Create(record).Error)

	return db, record
}

func certRecordCount(t *testing.T, db *gorm.DB, record *model.Cert) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Model(&model.Cert{}).Where("id = ?", record.ID).Count(&count).Error)
	return count
}

func TestRevokeCertKeepsRecordWhenRevocationFails(t *testing.T) {
	db, record := createUnrevokableCert(t)

	statuses := revokeCertStatuses(t, record, "")

	assert.Contains(t, statuses, Error)
	assert.NotContains(t, statuses, Success, "a failed revocation must not be reported as successful")
	assert.Equal(t, int64(1), certRecordCount(t, db, record), "the certificate record must survive a failed revocation")
}

func TestRevokeCertDeletesRecordOnFailureWhenRequested(t *testing.T) {
	db, record := createUnrevokableCert(t)

	statuses := revokeCertStatuses(t, record, "delete_on_failure=true")

	assert.Equal(t, []string{Warning}, statuses, "a deletion after a failed revocation must be reported once, as a warning")
	assert.Equal(t, int64(0), certRecordCount(t, db, record), "the record must be deleted when requested despite the failed revocation")
}
