package certificate

import (
	"errors"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

type RevokeCertResponse struct {
	Status string `json:"status"`
	*translation.Container
}

func handleRevokeCertLogChan(writer *helper.SafeWebSocketWriter, logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	for logString := range logChan {
		err := writer.WriteJSON(RevokeCertResponse{
			Status:    Info,
			Container: translation.C(logString),
		})
		if err != nil {
			logger.Error(err)
			return
		}
	}
}

// RevokeCert handles certificate revocation through websocket connection.
// With delete_on_failure=true the record is removed even if the CA did not
// revoke the certificate, and the outcome is reported as a warning.
func RevokeCert(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))
	deleteOnFailure := cast.ToBool(c.Query("delete_on_failure"))

	var upGrader = websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
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

	wsWriter := helper.NewSafeWebSocketWriter(ws)

	// Get certificate from database
	certQuery := query.Cert
	certModel, err := certQuery.FirstByID(id)
	if err != nil {
		logger.Error(err)
		_ = wsWriter.WriteJSON(RevokeCertResponse{
			Status: Error,
			Container: translation.C("Certificate not found: %{error}", map[string]any{
				"error": err.Error(),
			}),
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

	certLogger := cert.NewLogger()
	certLogger.SetWebSocket(wsWriter)
	defer certLogger.Close()

	go cert.RevokeCert(payload, certLogger, logChan, errChan)

	go handleRevokeCertLogChan(wsWriter, logChan)

	// block, until errChan closes
	var revokeErr error
	for err = range errChan {
		logger.Error(err)
		revokeErr = errors.Join(revokeErr, err)
	}

	// The CA did not revoke the certificate, so keep the record unless the
	// user asked to remove it anyway.
	if revokeErr != nil && !deleteOnFailure {
		_ = wsWriter.WriteJSON(RevokeCertResponse{
			Status: Error,
			Container: translation.C("Failed to revoke certificate: %{error}", map[string]any{
				"error": revokeErr.Error(),
			}),
		})
		return
	}

	// Update certificate status in database
	err = certModel.Remove()
	if err != nil {
		logger.Error(err)
		_ = wsWriter.WriteJSON(RevokeCertResponse{
			Status: Error,
			Container: translation.C("Failed to delete certificate from database: %{error}", map[string]any{
				"error": err.Error(),
			}),
		})
		return
	}

	if revokeErr != nil {
		_ = wsWriter.WriteJSON(RevokeCertResponse{
			Status: Warning,
			Container: translation.C("Certificate deleted, but revocation failed: %{error}", map[string]any{
				"error": revokeErr.Error(),
			}),
		})
		return
	}

	err = wsWriter.WriteJSON(RevokeCertResponse{
		Status:    Success,
		Container: translation.C("Certificate revoked successfully"),
	})
	if err != nil {
		logger.Error(err)
		return
	}
}
