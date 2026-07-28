package cert

import (
	"math"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

type expiryNotificationStage string

const (
	expiryStageNotice   expiryNotificationStage = "notice"
	expiryStageWarning  expiryNotificationStage = "warning"
	expiryStageUrgent   expiryNotificationStage = "urgent"
	expiryStageCritical expiryNotificationStage = "critical"
	expiryStageExpired  expiryNotificationStage = "expired"
)

type expiryNotification struct {
	Stage   expiryNotificationStage
	Type    model.NotificationType
	Title   string
	Content string
	Details map[string]any
}

func ExpiredNotify() {
	c := query.Cert
	certs, err := c.Find()
	if err != nil {
		logger.Errorf("ExpiredNotify: Err: %v\n", err)
		return
	}

	now := time.Now()
	for _, certModel := range certs {
		if certModel.SSLCertificatePath == "" {
			continue
		}

		certInfo, infoErr := GetCertInfo(certModel.SSLCertificatePath)
		if infoErr != nil {
			continue
		}

		notice := buildExpiryNotification(certModel, certInfo, now)
		if notice == nil {
			continue
		}

		sendExpiryNotification(notice)
		updateExpiryNotificationState(certModel, certInfo.NotAfter, notice.Stage, now)
	}
}

func buildExpiryNotification(certModel *model.Cert, info *Info, now time.Time) *expiryNotification {
	if certModel == nil || info == nil || !info.NotAfter.After(info.NotBefore) {
		return nil
	}

	remaining := info.NotAfter.Sub(now)
	stage := expiryNotificationStage("")
	if remaining <= 0 {
		if remaining < -24*time.Hour {
			return nil
		}
		stage = expiryStageExpired
	} else if isShortLivedCertificate(info) {
		if certModel.NextAutoRenewAt != nil && now.Before(*certModel.NextAutoRenewAt) {
			return nil
		}
		stage = shortLivedExpiryStage(info.NotAfter.Sub(info.NotBefore), remaining)
	} else {
		stage = standardExpiryStage(remaining)
	}
	if stage == "" || expiryNotificationAlreadySent(certModel, info.NotAfter, stage) {
		return nil
	}

	details := map[string]any{
		"name":      getAutoRenewTargetName(certModel),
		"not_after": info.NotAfter,
		"stage":     stage,
	}
	if stage == expiryStageExpired {
		return &expiryNotification{
			Stage:   stage,
			Type:    model.NotificationError,
			Title:   "Certificate Expired",
			Content: "Certificate %{name} has expired",
			Details: details,
		}
	}

	notice := &expiryNotification{
		Stage:   stage,
		Type:    expiryNotificationType(stage),
		Title:   expiryNotificationTitle(stage),
		Details: details,
	}
	days := int(math.Ceil(remaining.Hours() / 24))
	if days <= 1 {
		notice.Content = "Certificate %{name} will expire in 1 day"
	} else {
		notice.Content = "Certificate %{name} will expire in %{days} days"
		details["days"] = days
	}
	return notice
}

func shortLivedExpiryStage(validity, remaining time.Duration) expiryNotificationStage {
	criticalThreshold := validity / 24
	if criticalThreshold < 6*time.Hour {
		criticalThreshold = 6 * time.Hour
	}

	switch {
	case remaining <= criticalThreshold:
		return expiryStageCritical
	case remaining <= validity/8:
		return expiryStageUrgent
	case remaining <= validity/4:
		return expiryStageNotice
	default:
		return ""
	}
}

func standardExpiryStage(remaining time.Duration) expiryNotificationStage {
	switch {
	case remaining <= 24*time.Hour:
		return expiryStageCritical
	case remaining <= 3*24*time.Hour:
		return expiryStageUrgent
	case remaining <= 7*24*time.Hour:
		return expiryStageWarning
	case remaining <= 14*24*time.Hour:
		return expiryStageNotice
	default:
		return ""
	}
}

func expiryNotificationAlreadySent(certModel *model.Cert, notAfter time.Time,
	stage expiryNotificationStage) bool {
	if certModel.LastExpiryNotifyNotAfter == nil ||
		!certModel.LastExpiryNotifyNotAfter.Equal(notAfter) {
		return false
	}
	return expiryNotificationStageRank(expiryNotificationStage(certModel.LastExpiryNotifyStage)) >=
		expiryNotificationStageRank(stage)
}

func expiryNotificationStageRank(stage expiryNotificationStage) int {
	switch stage {
	case expiryStageNotice:
		return 1
	case expiryStageWarning:
		return 2
	case expiryStageUrgent:
		return 3
	case expiryStageCritical:
		return 4
	case expiryStageExpired:
		return 5
	default:
		return 0
	}
}

func expiryNotificationType(stage expiryNotificationStage) model.NotificationType {
	switch stage {
	case expiryStageCritical, expiryStageExpired:
		return model.NotificationError
	case expiryStageWarning, expiryStageUrgent:
		return model.NotificationWarning
	default:
		return model.NotificationInfo
	}
}

func expiryNotificationTitle(stage expiryNotificationStage) string {
	if stage == expiryStageNotice {
		return "Certificate Expiration Notice"
	}
	return "Certificate Expiring Soon"
}

func sendExpiryNotification(notice *expiryNotification) {
	switch notice.Type {
	case model.NotificationError:
		notification.Error(notice.Title, notice.Content, notice.Details)
	case model.NotificationWarning:
		notification.Warning(notice.Title, notice.Content, notice.Details)
	default:
		notification.Info(notice.Title, notice.Content, notice.Details)
	}
}

func updateExpiryNotificationState(certModel *model.Cert, notAfter time.Time,
	stage expiryNotificationStage, notifiedAt time.Time) {
	certModel.LastExpiryNotifyAt = &notifiedAt
	certModel.LastExpiryNotifyNotAfter = &notAfter
	certModel.LastExpiryNotifyStage = string(stage)

	db := model.UseDB()
	if db == nil || certModel.ID == 0 {
		return
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", certModel.ID).Updates(map[string]any{
		"last_expiry_notify_at":        notifiedAt,
		"last_expiry_notify_not_after": notAfter,
		"last_expiry_notify_stage":     string(stage),
	}).Error; err != nil {
		logger.Error(err)
	}
}
