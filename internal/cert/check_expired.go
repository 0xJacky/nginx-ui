package cert

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

func CertExpiredNotify() {
	c := query.Cert

	certs, err := c.Find()
	if err != nil {
		logger.Errorf("CertExpiredNotify: Err: %v\n", err)
		return
	}

	for _, certModel := range certs {
		if certModel.SSLCertificatePath == "" {
			continue
		}

		certInfo, err := GetCertInfo(certModel.SSLCertificatePath)
		if err != nil {
			continue
		}

		now := time.Now()

		// Calculate days until expiration
		daysUntilExpiration := int(certInfo.NotAfter.Sub(now).Hours() / 24)

		// ignore expired certificate
		if daysUntilExpiration < -1 {
			continue
		}

		mask := map[string]any{
			"name": certModel.Name,
			"days": daysUntilExpiration,
		}

		// Check if certificate is already expired
		if now.After(certInfo.NotAfter) {
			notification.Error("Certificate Expired", "Certificate %{name} has expired", mask)
			continue
		}

		// Send notifications based on remaining days
		switch {
		case daysUntilExpiration <= 14:
			notification.Info("Certificate Expiration Notice",
				"Certificate %{name} will expire in %{days} days", mask)
		case daysUntilExpiration <= 7:
			notification.Warning("Certificate Expiring Soon",
				"Certificate %{name} will expire in %{days} days", mask)
		case daysUntilExpiration <= 3:
			notification.Warning("Certificate Expiring Soon",
				"Certificate %{name} will expire in %{days} days", mask)
		case daysUntilExpiration <= 1:
			notification.Error("Certificate Expiring Soon",
				"Certificate %{name} will expire in 1 day", mask)
		}
	}
}
