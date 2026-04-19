package cert

import (
	stderrors "errors"
	"runtime"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	pkgerrors "github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	autoRenewFailureRetryCooldown = 12 * time.Hour
)

func AutoCert() {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
		}
	}()
	logger.Info("AutoCert Worker Started")
	autoCertList := model.GetAutoCertList()
	for _, certModel := range autoCertList {
		autoCert(certModel)
	}
	logger.Info("AutoCert Worker End")
}

func autoCert(certModel *model.Cert) {
	log := NewLogger()
	log.SetCertModel(certModel)
	defer log.Close()

	targetName := getAutoRenewTargetName(certModel)
	now := time.Now()

	if shouldSkipAutoRenew(certModel, now) {
		logger.Infof("Skip auto renew for %s until %s after previous failure", targetName,
			certModel.LastAutoRenewAt.Add(autoRenewFailureRetryCooldown).Format(time.DateTime))
		return
	}

	if len(certModel.Filename) == 0 {
		handleAutoRenewFailure(certModel, log, targetName, ErrCertModelFilenameEmpty)
		return
	}

	if len(certModel.Domains) == 0 {
		handleAutoRenewFailure(certModel, log, targetName,
			pkgerrors.New("domains list is empty, try to reopen auto-cert for this config:"+certModel.Filename))
		return
	}

	if certModel.SSLCertificatePath == "" {
		handleAutoRenewFailure(certModel, log, targetName,
			pkgerrors.New("ssl certificate path is empty, try to reopen auto-cert for this config:"+certModel.Filename))
		return
	}

	certInfo, err := GetCertInfo(certModel.SSLCertificatePath)
	if err != nil {
		handleAutoRenewFailure(certModel, log, targetName, pkgerrors.Wrap(err, "get certificate info error"))
		return
	}

	// Calculate certificate age (days since NotBefore)
	certAge := int(time.Since(certInfo.NotBefore).Hours() / 24)
	// Calculate days until expiration
	daysUntilExpiration := int(time.Until(certInfo.NotAfter).Hours() / 24)
	// Calculate total certificate validity period
	totalValidityDays := int(certInfo.NotAfter.Sub(certInfo.NotBefore).Hours() / 24)

	renewalInterval := settings.CertSettings.GetCertRenewalInterval()

	// For certificates with short validity periods (less than renewal interval),
	// use early renewal logic to prevent expiration.
	if totalValidityDays < renewalInterval {
		// Renew when 2/3 of the certificate's lifetime remains.
		earlyRenewalThreshold := 2 * totalValidityDays / 3
		if daysUntilExpiration > earlyRenewalThreshold {
			return
		}
	} else {
		// For normal certificates with validity >= renewal interval:
		// skip renewal if certificate age is less than the configured renewal interval.
		if certAge < renewalInterval {
			return
		}
	}

	payload := &ConfigPayload{
		CertID:                  certModel.ID,
		ServerName:              certModel.Domains,
		ChallengeMethod:         certModel.ChallengeMethod,
		DNSCredentialID:         certModel.DnsCredentialID,
		KeyType:                 certModel.GetKeyType(),
		ACMEUserID:              certModel.ACMEUserID,
		NotBefore:               certInfo.NotBefore,
		MustStaple:              certModel.MustStaple,
		LegoDisableCNAMESupport: certModel.LegoDisableCNAMESupport,
		RevokeOld:               certModel.RevokeOld,
	}

	if certModel.Resource != nil {
		payload.Resource = &model.CertificateResource{
			Resource:          certModel.Resource.Resource,
			PrivateKey:        certModel.Resource.PrivateKey,
			Certificate:       certModel.Resource.Certificate,
			IssuerCertificate: certModel.Resource.IssuerCertificate,
			CSR:               certModel.Resource.CSR,
		}
	}

	err = IssueCert(payload, log)
	if err != nil {
		handleAutoRenewFailure(certModel, log, targetName, err)
		return
	}

	updateAutoRenewStatus(certModel, now, "")
	notification.Success("Renew Certificate Success", "Certificate %{name} renewed successfully", map[string]any{
		"name": targetName,
	})

	err = SyncToRemoteServer(certModel)
	if err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
		return
	}
}

func shouldSkipAutoRenew(certModel *model.Cert, now time.Time) bool {
	if certModel == nil || certModel.LastAutoRenewAt == nil || certModel.LastAutoRenewError == "" {
		return false
	}

	return now.Before(certModel.LastAutoRenewAt.Add(autoRenewFailureRetryCooldown))
}

func handleAutoRenewFailure(certModel *model.Cert, log *Logger, name string, err error) {
	log.Error(err)
	updateAutoRenewStatus(certModel, time.Now(), err.Error())
	notification.Error("Renew Certificate Error", "Certificate %{name} renewal failed: %{error}",
		buildAutoRenewNotificationDetails(name, err))
}

func updateAutoRenewStatus(certModel *model.Cert, at time.Time, renewalError string) {
	if certModel == nil {
		return
	}

	certModel.LastAutoRenewAt = &at
	certModel.LastAutoRenewError = renewalError

	db := model.UseDB()
	if db == nil || certModel.ID == 0 {
		return
	}

	err := db.Model(&model.Cert{}).
		Where("id = ?", certModel.ID).
		Updates(map[string]any{
			"last_auto_renew_at":    at,
			"last_auto_renew_error": renewalError,
		}).Error
	if err != nil {
		logger.Error(err)
	}
}

func buildAutoRenewNotificationDetails(name string, err error) map[string]any {
	details := map[string]any{
		"name": name,
	}

	if err == nil {
		return details
	}

	details["error"] = strings.TrimSpace(err.Error())
	details["response"] = getAutoRenewNotificationResponse(err)

	return details
}

func getAutoRenewNotificationResponse(err error) any {
	if err == nil {
		return nil
	}

	var cosyErr *cosy.Error
	if stderrors.As(err, &cosyErr) {
		return cosyErr
	}

	return strings.TrimSpace(err.Error())
}

func getAutoRenewTargetName(certModel *model.Cert) string {
	if certModel == nil {
		return "unknown certificate"
	}

	if len(certModel.Domains) > 0 {
		return strings.Join(certModel.Domains, ", ")
	}

	if certModel.Filename != "" {
		return certModel.Filename
	}

	if certModel.Name != "" {
		return certModel.Name
	}

	return "unknown certificate"
}
