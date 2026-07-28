package cert

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	stderrors "errors"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/acme/api"
	"github.com/go-acme/lego/v5/lego"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
)

const (
	renewalInfoRefreshInterval = 6 * time.Hour
	renewalInfoRequestTimeout  = 30 * time.Second
)

type renewalScheduleDecision struct {
	Due            bool
	UsesARI        bool
	ReplacesCertID string
}

func getRenewalScheduleDecision(certModel *model.Cert, certificate *x509.Certificate,
	now time.Time) renewalScheduleDecision {
	if certModel == nil || certificate == nil || !isShortLivedCertificate(certificateInfo(certificate)) {
		return renewalScheduleDecision{}
	}

	fingerprint := certificateFingerprint(certificate)
	if certModel.AutoRenewScheduleFingerprint != fingerprint {
		clearAutoRenewSchedule(certModel)
	}

	if certModel.LastRenewalInfoCheckAt == nil ||
		now.Sub(*certModel.LastRenewalInfoCheckAt) >= renewalInfoRefreshInterval {
		refreshAutoRenewSchedule(certModel, certificate, now, fingerprint)
	}

	if certModel.NextAutoRenewAt == nil {
		return renewalScheduleDecision{}
	}

	decision := renewalScheduleDecision{UsesARI: true}
	if now.Before(*certModel.NextAutoRenewAt) {
		return decision
	}

	decision.Due = true
	certID, err := api.MakeARICertID(certificate)
	if err != nil {
		logger.Warnf("Build ARI certificate ID: %v", err)
		return decision
	}
	decision.ReplacesCertID = certID
	return decision
}

func refreshAutoRenewSchedule(certModel *model.Cert, certificate *x509.Certificate,
	now time.Time, fingerprint string) {
	renewAt, err := fetchARIRenewalTime(certModel, certificate, now)
	if err != nil {
		if stderrors.Is(err, api.ErrNoARI) {
			logger.Debugf("ACME server does not advertise ARI for certificate %s", getAutoRenewTargetName(certModel))
		} else {
			logger.Warnf("Fetch ARI renewal schedule for %s: %v", getAutoRenewTargetName(certModel), err)
		}
		updateAutoRenewSchedule(certModel, now, nil, fingerprint, false)
		return
	}

	updateAutoRenewSchedule(certModel, now, &renewAt, fingerprint, true)
}

func fetchARIRenewalTime(certModel *model.Cert, certificate *x509.Certificate,
	now time.Time) (time.Time, error) {
	payload := &ConfigPayload{ACMEUserID: certModel.ACMEUserID}
	user, err := payload.GetACMEUser()
	if err != nil {
		return time.Time{}, errors.Wrap(err, "get ACME user")
	}

	config := lego.NewConfig(user)
	config.CADirURL = user.CADir
	if config.HTTPClient != nil {
		clientTransport, transportErr := transport.NewTransport(transport.WithProxy(user.Proxy))
		if transportErr != nil {
			return time.Time{}, errors.Wrap(transportErr, "create ACME transport")
		}
		config.HTTPClient.Transport = clientTransport
	}

	client, err := lego.NewClient(config)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "create ACME client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), renewalInfoRequestTimeout)
	defer cancel()
	renewalInfo, err := client.Certificate.GetRenewalInfo(ctx, certificate)
	if err != nil {
		return time.Time{}, err
	}

	return selectARIRenewalTime(certificate, renewalInfo.SuggestedWindow.Start,
		renewalInfo.SuggestedWindow.End, now)
}

func selectARIRenewalTime(certificate *x509.Certificate, start, end, now time.Time) (time.Time, error) {
	if certificate == nil {
		return time.Time{}, errors.New("certificate is nil")
	}
	if end.Before(start) || end.Equal(start) {
		return time.Time{}, errors.New("ARI suggested renewal window is invalid")
	}

	window := end.Sub(start)
	sum := sha256.Sum256(certificate.Raw)
	offset := time.Duration(binary.BigEndian.Uint64(sum[:8]) % uint64(window))
	renewAt := start.Add(offset)
	if renewAt.Before(now) {
		return now, nil
	}
	return renewAt, nil
}

func updateAutoRenewSchedule(certModel *model.Cert, checkedAt time.Time, renewAt *time.Time,
	fingerprint string, replaceSchedule bool) {
	certModel.LastRenewalInfoCheckAt = &checkedAt
	certModel.AutoRenewScheduleFingerprint = fingerprint
	updates := map[string]any{
		"last_renewal_info_check_at":      checkedAt,
		"auto_renew_schedule_fingerprint": fingerprint,
	}
	if replaceSchedule {
		certModel.NextAutoRenewAt = renewAt
		updates["next_auto_renew_at"] = renewAt
	}

	db := model.UseDB()
	if db == nil || certModel.ID == 0 {
		return
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", certModel.ID).Updates(updates).Error; err != nil {
		logger.Error(err)
	}
}

func clearAutoRenewSchedule(certModel *model.Cert) {
	if certModel == nil {
		return
	}
	certModel.NextAutoRenewAt = nil
	certModel.LastRenewalInfoCheckAt = nil
	certModel.AutoRenewScheduleFingerprint = ""

	db := model.UseDB()
	if db == nil || certModel.ID == 0 {
		return
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", certModel.ID).Updates(map[string]any{
		"next_auto_renew_at":              nil,
		"last_renewal_info_check_at":      nil,
		"auto_renew_schedule_fingerprint": "",
	}).Error; err != nil {
		logger.Error(err)
	}
}
