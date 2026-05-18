package cert

import (
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	pkgerrors "github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
)

// RenewSelfSignedCerts renews every self-signed certificate that is close to
// expiry. It is invoked by a dedicated cron job.
func RenewSelfSignedCerts() {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
		}
	}()
	logger.Info("RenewSelfSignedCerts Worker Started")

	db := model.UseDB()
	if db == nil {
		return
	}

	var certs []*model.Cert
	db.Where("auto_cert = ?", model.AutoCertSelfSigned).Find(&certs)

	now := time.Now()
	renewalInterval := settings.CertSettings.GetCertRenewalInterval()
	for _, certModel := range certs {
		renewSelfSignedCert(certModel, now, renewalInterval)
	}
	logger.Info("RenewSelfSignedCerts Worker End")
}

// renewSelfSignedCert renews a single self-signed certificate when it is due.
func renewSelfSignedCert(certModel *model.Cert, now time.Time, renewalInterval int) {
	log := NewLogger()
	log.SetCertModel(certModel)
	defer log.Close()

	targetName := getAutoRenewTargetName(certModel)

	if shouldSkipAutoRenew(certModel, now) {
		logger.Infof("Skip auto renew for %s until %s after previous failure", targetName,
			certModel.LastAutoRenewAt.Add(autoRenewFailureRetryCooldown).Format(time.DateTime))
		return
	}

	if certModel.SSLCertificatePath == "" {
		handleAutoRenewFailure(certModel, log, targetName,
			pkgerrors.New("ssl certificate path is empty for self-signed certificate"))
		return
	}

	info, err := GetCertInfo(certModel.SSLCertificatePath)
	if err != nil {
		handleAutoRenewFailure(certModel, log, targetName,
			pkgerrors.Wrap(err, "get self-signed certificate info error"))
		return
	}

	if !shouldRenewSelfSignedCert(info, now, renewalInterval) {
		return
	}

	certPEM, keyPEM, err := RegenerateSelfSigned(certModel)
	if err != nil {
		handleAutoRenewFailure(certModel, log, targetName, err)
		return
	}

	content := &Content{
		SSLCertificatePath:    certModel.SSLCertificatePath,
		SSLCertificateKeyPath: certModel.SSLCertificateKeyPath,
		SSLCertificate:        string(certPEM),
		SSLCertificateKey:     string(keyPEM),
	}
	if err = content.WriteFile(); err != nil {
		handleAutoRenewFailure(certModel, log, targetName, err)
		return
	}

	nginx.Reload()

	updateAutoRenewStatus(certModel, now, "")
	notification.Success("Renew Certificate Success",
		"Certificate %{name} renewed successfully", map[string]any{"name": targetName})

	if err = SyncToRemoteServer(certModel); err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
	}
}

// shouldRenewSelfSignedCert reports whether a self-signed certificate with the
// given info should be renewed now. It mirrors the renewal-threshold logic of
// the ACME auto-renewal job.
func shouldRenewSelfSignedCert(info *Info, now time.Time, renewalInterval int) bool {
	certAge := int(now.Sub(info.NotBefore).Hours() / 24)
	daysUntilExpiration := int(info.NotAfter.Sub(now).Hours() / 24)
	totalValidityDays := int(info.NotAfter.Sub(info.NotBefore).Hours() / 24)

	if totalValidityDays < renewalInterval {
		// short-lived certificate: renew once 2/3 of the lifetime has elapsed
		earlyRenewalThreshold := 2 * totalValidityDays / 3
		return daysUntilExpiration <= earlyRenewalThreshold
	}
	// normal certificate: renew once the age reaches the renewal interval
	return certAge >= renewalInterval
}
