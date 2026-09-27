package certificate

import (
	"context"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

const (
	Success = "success"
	Info    = "info"
	Warning = "warning"
	Error   = "error"
)

type IssueCertResponse struct {
	Status            string             `json:"status"`
	Message           string             `json:"message"`
	SSLCertificate    string             `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string             `json:"ssl_certificate_key,omitempty"`
	KeyType           certcrypto.KeyType `json:"key_type,omitempty"`
	Profile           string             `json:"profile,omitempty"`
	// Hint explains an issuance failure in actionable terms. Only set on errors.
	Hint *acmehint.Hint `json:"hint,omitempty"`
}

// issueHintDiagnoseTimeout bounds the DNS evidence collection that runs after
// a failed issuance, so a slow resolver cannot delay the error response.
const issueHintDiagnoseTimeout = 4 * time.Second

// diagnoseForHint is a seam for tests.
var diagnoseForHint = acmehint.Diagnose

// issueFailureHint classifies an issuance error. For HTTP-01 it first
// resolves the identifiers so the classifier can tell "DNS points to another
// server" apart from "this server did not route the challenge".
func issueFailureHint(ctx context.Context, payload *cert.ConfigPayload, err error) *acmehint.Hint {
	if err == nil {
		return nil
	}
	var ev acmehint.Evidence
	if payload.ChallengeMethod == "" || payload.ChallengeMethod == cert.HTTP01 {
		dctx, cancel := context.WithTimeout(ctx, issueHintDiagnoseTimeout)
		defer cancel()
		ev = acmehint.EvidenceFromDiagnostics(diagnoseForHint(dctx, payload.ServerName, acmehint.Options{
			LookupTimeout: issueHintDiagnoseTimeout / 2,
		}))
	}
	return acmehint.Classify(err, ev)
}

func IssueCert(c *gin.Context) {
	name := c.Param("name")
	var upGrader = websocket.Upgrader{
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
		logger.Error(err)
		_ = wsWriter.WriteJSON(IssueCertResponse{Status: Error, Message: err.Error()})
		return
	}

	payload.ConfigName = name

	log := cert.NewLogger()
	log.SetWebSocket(wsWriter)
	defer log.Close()

	certModel, err := cert.IssueWithRecord(name, payload, log)
	if certModel == nil && err != nil {
		logger.Error(err)
		_ = wsWriter.WriteJSON(IssueCertResponse{Status: Error, Message: err.Error()})
		return
	}
	if err != nil {
		hint := issueFailureHint(c.Request.Context(), payload, err)
		_ = wsWriter.WriteJSON(IssueCertResponse{Status: Error, Message: err.Error(), Hint: hint})
		return
	}

	if err := wsWriter.WriteJSON(IssueCertResponse{
		Status:            Success,
		Message:           translation.C("[Nginx UI] Issued certificate successfully").ToString(),
		SSLCertificate:    payload.GetCertificatePath(),
		SSLCertificateKey: payload.GetCertificateKeyPath(),
		KeyType:           payload.GetKeyType(),
		Profile:           payload.Profile,
	}); err != nil {
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
		}
	}
}

// The cert record bookkeeping lives in internal/cert so the HTTPS onboarding
// orchestrator shares it with this endpoint.
var (
	persistCertDraft = cert.PersistCertDraft
	markCertFailure  = cert.MarkCertFailure
	markCertSuccess  = cert.MarkCertSuccess
	shortError       = cert.ShortError
)
