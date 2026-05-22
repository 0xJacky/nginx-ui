package certificate

import (
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

const (
	Success = "success"
	Info    = "info"
	Error   = "error"
)

type IssueCertResponse struct {
	Status            string             `json:"status"`
	Message           string             `json:"message"`
	SSLCertificate    string             `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string             `json:"ssl_certificate_key,omitempty"`
	KeyType           certcrypto.KeyType `json:"key_type,omitempty"`
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
	payload.KeyType = payload.GetKeyType()

	certModel, err := persistCertDraft(name, payload)
	if err != nil {
		logger.Error(err)
		_ = wsWriter.WriteJSON(IssueCertResponse{Status: Error, Message: err.Error()})
		return
	}

	payload.CertID = certModel.ID

	// Defer guard: if the function returns while still pending (panic / unexpected path),
	// the record would otherwise be orphaned. Convert to failure with a generic message.
	defer func() {
		var current model.Cert
		db := model.UseDB()
		if db == nil {
			return
		}
		if e := db.Where("id = ?", certModel.ID).First(&current).Error; e != nil {
			return
		}
		if current.Status == model.CertStatusPending {
			markCertFailure(certModel.ID, "Issuance interrupted before completion.")
		}
	}()

	// Hydrate payload.Resource from the existing cert (for renewal path).
	if certModel.SSLCertificatePath != "" {
		certInfo, _ := cert.GetCertInfo(certModel.SSLCertificatePath)
		if certInfo != nil {
			payload.Resource = certModel.Resource
			payload.NotBefore = certInfo.NotBefore
		}
	}

	log := cert.NewLogger()
	log.SetCertModel(certModel)
	log.SetWebSocket(wsWriter)
	defer log.Close()

	if err := cert.IssueCert(payload, log); err != nil {
		log.Error(err)
		markCertFailure(certModel.ID, shortError(err))
		_ = wsWriter.WriteJSON(IssueCertResponse{Status: Error, Message: err.Error()})
		return
	}

	markCertSuccess(certModel.ID, payload.GetCertificatePath(), payload.GetCertificateKeyPath(), payload.Resource)

	if err := wsWriter.WriteJSON(IssueCertResponse{
		Status:            Success,
		Message:           translation.C("[Nginx UI] Issued certificate successfully").ToString(),
		SSLCertificate:    payload.GetCertificatePath(),
		SSLCertificateKey: payload.GetCertificateKeyPath(),
		KeyType:           payload.GetKeyType(),
	}); err != nil {
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
		}
	}
}

// persistCertDraft inserts or updates a Cert row representing an in-flight issuance.
// The row is keyed by (name, filename, key_type). All user-submitted config is captured
// up-front so a failure preserves enough state for a one-click retry.
func persistCertDraft(name string, payload *cert.ConfigPayload) (*model.Cert, error) {
	db := model.UseDB()
	normalizedKeyType := helper.GetKeyType(payload.GetKeyType())
	keyTypeAliases := helper.GetKeyTypeAliasStrings(normalizedKeyType)

	now := time.Now()

	seed := &model.Cert{
		Name:                    name,
		Filename:                name,
		KeyType:                 normalizedKeyType,
		Domains:                 payload.ServerName,
		ChallengeMethod:         payload.ChallengeMethod,
		DnsCredentialID:         payload.DNSCredentialID,
		ACMEUserID:              payload.ACMEUserID,
		AutoCert:                model.AutoCertEnabled,
		MustStaple:              payload.MustStaple,
		LegoDisableCNAMESupport: payload.LegoDisableCNAMESupport,
		RevokeOld:               payload.RevokeOld,
		Status:                  model.CertStatusPending,
		LastError:               "",
		LastAttemptAt:           &now,
	}

	// FirstOrCreate by (name, filename, key_type). When the row exists,
	// `seed` is hydrated with the existing record (preserving SSLCertificatePath,
	// Resource, etc.) so we can read those fields on the renewal path below.
	if err := db.Where("name = ? AND filename = ? AND key_type IN ?", name, name, keyTypeAliases).
		FirstOrCreate(seed).Error; err != nil {
		return nil, err
	}

	// Refresh all user-submitted config and reset issuance state to pending.
	// Use struct + Select so GORM applies the `serializer:json` tag for Domains
	// AND writes the zero-valued LastError ("") instead of skipping it.
	updates := &model.Cert{
		Domains:                 payload.ServerName,
		ChallengeMethod:         payload.ChallengeMethod,
		DnsCredentialID:         payload.DNSCredentialID,
		ACMEUserID:              payload.ACMEUserID,
		AutoCert:                model.AutoCertEnabled,
		MustStaple:              payload.MustStaple,
		LegoDisableCNAMESupport: payload.LegoDisableCNAMESupport,
		RevokeOld:               payload.RevokeOld,
		Status:                  model.CertStatusPending,
		LastError:               "",
		LastAttemptAt:           &now,
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", seed.ID).
		Select(
			"domains", "challenge_method", "dns_credential_id", "acme_user_id",
			"auto_cert", "must_staple", "lego_disable_cname_support",
			"revoke_old", "status", "last_error", "last_attempt_at",
		).
		Updates(updates).Error; err != nil {
		return nil, err
	}

	// Re-read so the caller has the fully-populated struct (Resource, paths, etc.).
	var fresh model.Cert
	if err := db.Where("id = ?", seed.ID).First(&fresh).Error; err != nil {
		return nil, err
	}
	return &fresh, nil
}

// markCertFailure updates only the failure-related columns. It explicitly
// avoids touching SSLCertificatePath / SSLCertificateKeyPath / Resource so
// a renew failure does not destroy the previously-issued certificate.
// Map-based Updates is safe here because neither column has a serializer tag.
func markCertFailure(id uint64, lastError string) {
	db := model.UseDB()
	if db == nil {
		return
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", id).Updates(map[string]any{
		"status":     model.CertStatusFailure,
		"last_error": lastError,
	}).Error; err != nil {
		logger.Errorf("markCertFailure: %v", err)
	}
}

// markCertSuccess updates the cert with the freshly-issued paths and Resource,
// flips status to success, and clears any prior last_error. Uses struct + Select
// so GORM applies the `serializer:json[aes]` tag for Resource AND writes the
// zero-valued LastError ("").
func markCertSuccess(id uint64, sslCertificatePath, sslCertificateKeyPath string, resource *model.CertificateResource) {
	db := model.UseDB()
	if db == nil {
		return
	}
	updates := &model.Cert{
		SSLCertificatePath:    sslCertificatePath,
		SSLCertificateKeyPath: sslCertificateKeyPath,
		Resource:              resource,
		Status:                model.CertStatusSuccess,
		LastError:             "",
	}
	cols := []string{"ssl_certificate_path", "ssl_certificate_key_path", "status", "last_error"}
	if resource != nil {
		cols = append(cols, "resource")
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", id).
		Select(cols).Updates(updates).Error; err != nil {
		logger.Errorf("markCertSuccess: %v", err)
	}
}

// shortError trims and truncates an error for UI display in last_error.
// Returns "" for nil so a successful retry can clear the prior error.
func shortError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	const max = 500
	if len(msg) > max {
		msg = msg[:max] + "…"
	}
	return msg
}
