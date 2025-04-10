package certificate

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

type RevokeCertResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func handleRevokeCertLogChan(conn *websocket.Conn, logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	for logString := range logChan {
		err := conn.WriteJSON(RevokeCertResponse{
			Status:  Info,
			Message: logString,
		})
		if err != nil {
			logger.Error(err)
			return
		}
	}
}

// RevokeCert handles certificate revocation through websocket connection
func RevokeCert(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	// Get certificate from database
	certQuery := query.Cert
	certModel, err := certQuery.FirstByID(id)
	if err != nil {
		logger.Error(err)
		_ = ws.WriteJSON(RevokeCertResponse{
			Status:  Error,
			Message: "Certificate not found: " + err.Error(),
		})
		return
	}

	// Create payload for revocation
	payload := &cert.ConfigPayload{
		CertID:          id,
		ServerName:      certModel.Domains,
		ChallengeMethod: certModel.ChallengeMethod,
		DNSCredentialID: certModel.DnsCredentialID,
		ACMEUserID:      certModel.ACMEUserID,
		KeyType:         certModel.KeyType,
		Resource:        certModel.Resource,
	}

	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go cert.RevokeCert(payload, logChan, errChan)

	go handleRevokeCertLogChan(ws, logChan)

	// block, until errChan closes
	for err = range errChan {
		logger.Error(err)
		err = ws.WriteJSON(RevokeCertResponse{
			Status:  Error,
			Message: err.Error(),
		})
		if err != nil {
			logger.Error(err)
			return
		}
		return
	}

	// Update certificate status in database
	err = certModel.Remove()
	if err != nil {
		logger.Error(err)
		_ = ws.WriteJSON(RevokeCertResponse{
			Status:  Error,
			Message: "Failed to delete certificate from database: " + err.Error(),
		})
		return
	}

	err = ws.WriteJSON(RevokeCertResponse{
		Status:  Success,
		Message: "Certificate revoked successfully",
	})
	if err != nil {
		logger.Error(err)
		return
	}
}
