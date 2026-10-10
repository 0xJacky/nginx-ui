package certificate

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

// IssueCertForNode issues a DNS-01 certificate for a configuration of a node
// with the DNS providers of this instance and sends it to the node. It speaks
// the protocol of IssueCert, and the paths it answers are the ones on the node.
func IssueCertForNode(c *gin.Context) {
	nodeID := cast.ToUint64(c.Param("id"))
	name := c.Param("name")
	upGrader := websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}

	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ws.Close()

	wsWriter := helper.NewSafeWebSocketWriter(ws)

	payload := &cert.ConfigPayload{}
	if err := ws.ReadJSON(payload); err != nil {
		logger.Error(err)
		return
	}
	stopKeepalive := startIssueCertKeepalive(ws, issueCertWSPingPeriod)
	defer stopKeepalive()

	payload.KeyType = payload.GetKeyType()
	if err := cert.NormalizeAndValidateIdentifiers(payload); err != nil {
		_ = wsWriter.WriteJSON(issueErrorResponse(err, nil))
		return
	}
	payload.ConfigName = name

	log := cert.NewLogger()
	log.SetWebSocket(wsWriter)
	defer log.Close()

	issued, err := cert.IssueForNode(c.Request.Context(), nodeID, name, payload, log)
	if err != nil {
		var hint = issueFailureHint(c.Request.Context(), payload, err)
		if issued == nil {
			hint = nil
		}
		_ = wsWriter.WriteJSON(issueErrorResponse(err, hint))
		return
	}

	if err := wsWriter.WriteJSON(IssueCertResponse{
		Status:              Success,
		Message:             translation.C("[Nginx UI] Issued certificate successfully").ToString(),
		SSLCertificate:      issued.Cert.RemoteSSLCertificatePath,
		SSLCertificateKey:   issued.Cert.RemoteSSLCertificateKeyPath,
		KeyType:             payload.GetKeyType(),
		Profile:             payload.Profile,
		RemoteCertificateID: issued.RemoteCertificateID,
	}); err != nil {
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
		}
	}
}
