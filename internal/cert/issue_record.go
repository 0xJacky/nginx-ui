package cert

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// issueInterruptedMessage is recorded when an issuance returns without
// reaching success or failure bookkeeping (panic or unexpected path).
const issueInterruptedMessage = "Issuance interrupted before completion."

// PersistCertDraft inserts or updates a Cert row representing an in-flight issuance.
// The row is keyed by (filename, key_type). All user-submitted config is captured
// up-front so a failure preserves enough state for a one-click retry.
func PersistCertDraft(name string, payload *ConfigPayload) (*model.Cert, error) {
	db := model.UseDB()
	if db == nil {
		return nil, ErrDatabaseNotInitialized
	}
	normalizedKeyType := helper.GetKeyType(payload.GetKeyType())
	keyTypeAliases := helper.GetKeyTypeAliasStrings(normalizedKeyType)
	certificateName := CertificateName(name, payload.ServerName)

	now := time.Now()

	seed := &model.Cert{
		Name:             certificateName,
		Filename:         name,
		KeyType:          normalizedKeyType,
		Domains:          payload.ServerName,
		ChallengeMethod:  payload.ChallengeMethod,
		Profile:          payload.Profile,
		DnsCredentialID:  payload.DNSCredentialID,
		ACMEUserID:       payload.ACMEUserID,
		AutoCert:         model.AutoCertEnabled,
		MustStaple:       payload.MustStaple,
		ChallengeConfig:  payload.ChallengeConfig,
		EnableCommonName: payload.EnableCommonName,
		RevokeOld:        payload.RevokeOld,
		Status:           model.CertStatusPending,
		LastError:        "",
		LastAttemptAt:    &now,
	}

	// FirstOrCreate by (filename, key_type). Name is the certificate identifier,
	// while Filename keeps the association with the site configuration.
	// When the row exists,
	// `seed` is hydrated with the existing record (preserving SSLCertificatePath,
	// Resource, etc.) so we can read those fields on the renewal path below.
	if err := db.Where("filename = ? AND key_type IN ?", name, keyTypeAliases).
		FirstOrCreate(seed).Error; err != nil {
		return nil, err
	}
	if payload.Profile == "" {
		payload.Profile = seed.Profile
		if payload.Profile == "" && seed.Resource != nil && seed.Resource.Resource != nil {
			payload.Profile = seed.Resource.Profile
		}
	}

	// Refresh all user-submitted config and reset issuance state to pending.
	// Use struct + Select so GORM applies the `serializer:json` tag for Domains
	// AND writes the zero-valued LastError ("") instead of skipping it.
	updates := &model.Cert{
		Name:             certificateName,
		Domains:          payload.ServerName,
		ChallengeMethod:  payload.ChallengeMethod,
		Profile:          payload.Profile,
		DnsCredentialID:  payload.DNSCredentialID,
		ACMEUserID:       payload.ACMEUserID,
		AutoCert:         model.AutoCertEnabled,
		MustStaple:       payload.MustStaple,
		ChallengeConfig:  payload.ChallengeConfig,
		EnableCommonName: payload.EnableCommonName,
		RevokeOld:        payload.RevokeOld,
		Status:           model.CertStatusPending,
		LastError:        "",
		LastAttemptAt:    &now,
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", seed.ID).
		Select(
			"name", "domains", "challenge_method", "profile", "dns_credential_id", "acme_user_id",
			"auto_cert", "must_staple", "challenge_config", "enable_common_name",
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

// MarkCertFailure updates only the failure-related columns. It explicitly
// avoids touching SSLCertificatePath / SSLCertificateKeyPath / Resource so
// a renew failure does not destroy the previously-issued certificate.
// Map-based Updates is safe here because neither column has a serializer tag.
func MarkCertFailure(id uint64, lastError string) {
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

// MarkCertSuccess updates the cert with the freshly-issued paths and Resource,
// flips status to success, and clears any prior last_error. Uses struct + Select
// so GORM applies the `serializer:json[aes]` tag for Resource AND writes the
// zero-valued LastError ("").
func MarkCertSuccess(id uint64, sslCertificatePath, sslCertificateKeyPath string,
	resource *model.CertificateResource, profile string) {
	db := model.UseDB()
	if db == nil {
		return
	}
	updates := &model.Cert{
		SSLCertificatePath:    sslCertificatePath,
		SSLCertificateKeyPath: sslCertificateKeyPath,
		Resource:              resource,
		Profile:               profile,
		Status:                model.CertStatusSuccess,
		LastError:             "",
	}
	cols := []string{"ssl_certificate_path", "ssl_certificate_key_path", "profile", "status", "last_error"}
	if resource != nil {
		cols = append(cols, "resource")
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", id).
		Select(cols).Updates(updates).Error; err != nil {
		logger.Errorf("markCertSuccess: %v", err)
	}
}

// FailPendingCert marks the record as failed when it is still pending, so an
// issuance that returned early (panic or unexpected path) is not orphaned.
func FailPendingCert(id uint64, lastError string) {
	db := model.UseDB()
	if db == nil {
		return
	}
	var current model.Cert
	if err := db.Where("id = ?", id).First(&current).Error; err != nil {
		return
	}
	if current.Status == model.CertStatusPending {
		MarkCertFailure(id, lastError)
	}
}

// ShortError trims and truncates an error for UI display in last_error.
// Returns "" for nil so a successful retry can clear the prior error.
// Truncation is rune-aware so non-ASCII error messages (e.g. localized
// ACME or DNS provider errors) cannot be split mid-rune.
func ShortError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	const maxRunes = 500
	runes := []rune(msg)
	if len(runes) > maxRunes {
		msg = string(runes[:maxRunes]) + "…"
	}
	return msg
}

// IssueWithRecord issues the certificate described by payload for the site
// configuration name and keeps its cert record in sync: a pending draft before
// issuance, then success (with the issued paths) or failure (with the error).
// A reissue writes over the files the record already owns. It returns the
// record, which is nil only when the draft could not be persisted.
func IssueWithRecord(name string, payload *ConfigPayload, log *Logger) (*model.Cert, error) {
	certModel, err := PersistCertDraft(name, payload)
	if err != nil {
		return nil, err
	}
	payload.CertID = certModel.ID
	defer FailPendingCert(certModel.ID, issueInterruptedMessage)

	// Hydrate payload.Resource from the existing cert (for renewal path).
	if certModel.SSLCertificatePath != "" {
		certInfo, _ := GetCertInfo(certModel.SSLCertificatePath)
		if certInfo != nil {
			payload.Resource = certModel.Resource
			payload.NotBefore = certInfo.NotBefore
		}
	}

	// Reissue over the files this record already owns. The certificate
	// management page renews without touching any site configuration, so a
	// path derived from the current identifiers and key type would leave every
	// vhost referencing the previous, expiring files.
	payload.UseExistingCertificatePaths(certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath)

	log.SetCertModel(certModel)

	if err := IssueCert(payload, log); err != nil {
		log.Error(err)
		MarkCertFailure(certModel.ID, ShortError(err))
		return certModel, err
	}

	MarkCertSuccess(certModel.ID, payload.GetCertificatePath(), payload.GetCertificateKeyPath(), payload.Resource, payload.Profile)
	notifyCertificateRelocated(getAutoRenewTargetName(certModel), certModel.SSLCertificatePath, payload.GetCertificatePath())
	event.PublishCertIssued(certModel.ID, certModel.Name, payload.ServerName, false)
	return certModel, nil
}

// NewStreamLogger returns a Logger that hands every message to sink instead
// of writing it to a websocket. The returned close function closes the logger
// (persisting the log on the cert record) and waits until every buffered
// message has reached sink.
func NewStreamLogger(sink func(message *translation.Container)) (*Logger, func()) {
	l := &Logger{
		msgCh: make(chan []byte, 100),
		done:  make(chan struct{}),
	}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for msg := range l.msgCh {
			var container translation.Container
			if err := json.Unmarshal(msg, &container); err != nil {
				continue
			}
			if sink != nil {
				sink(&container)
			}
		}
	}()
	return l, func() {
		l.Close()
		<-drained
	}
}
