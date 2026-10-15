package ingress

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// Reasons of the CertReady condition set by the worker. The certificate
// manager supplies the reasons of a certificate that is not ready.
const (
	applyReasonCertIssued      = "Issued"
	applyReasonCertExisting    = "Existing"
	applyReasonCertNotRequired = "NotRequired"
)

// maxConditionMessage caps the nginx output kept in a condition message.
const maxConditionMessage = 4000

func truncateMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= maxConditionMessage {
		return message
	}
	return message[:maxConditionMessage] + "…"
}

// recordApplyStatus writes the outcome of applying the snapshot ing into the
// stored status and wakes the waiters.
func recordApplyStatus(ing *model.Ingress, r *itemResult, cert CertState) {
	_, err := UpdateStatus(ing.Name, func(cur *model.Ingress) {
		st := &cur.Status
		st.Phase = ""
		if ing.Generation > st.ObservedGeneration {
			st.ObservedGeneration = ing.Generation
		}

		if r.failed {
			message := r.Error
			if r.TestOutput != "" && !strings.Contains(message, r.TestOutput) {
				message += "\n" + r.TestOutput
			}
			SetCondition(st, model.IngressCondition{
				Type:    model.IngressConditionConfigApplied,
				Status:  false,
				Reason:  r.Reason,
				Message: truncateMessage(message),
			})
		} else {
			SetCondition(st, model.IngressCondition{
				Type:   model.IngressConditionConfigApplied,
				Status: true,
				Reason: applyReasonApplied,
			})
			if r.Rendered != nil {
				st.ConfigHash = r.Rendered.Hash
			}
			if r.Changed || st.LastAppliedAt == nil {
				now := time.Now()
				st.LastAppliedAt = &now
			}
		}

		SetCondition(st, certCondition(ing, cert, time.Now()))
		if ing.Spec.TLS.Mode == model.IngressTLSModeNone {
			st.CertID = 0
			st.CertNotAfter = nil
		} else {
			st.CertID = cert.CertID
			st.CertNotAfter = cert.NotAfter
		}

		if _, ok := FindCondition(st, model.IngressConditionBackendsHealthy); !ok {
			SetCondition(st, model.IngressCondition{
				Type:   model.IngressConditionBackendsHealthy,
				Status: false,
				Reason: ReasonUnknown,
			})
		}
		syncServerStatus(st, &cur.Spec)
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Errorf("Recording the status of ingress %s failed: %v", ing.Name, err)
	}
	notifyWaiters(ing.Name)
}

// syncServerStatus lists every server of the spec in the status, keeping the
// health the reconcile loop recorded and taking down from the spec.
func syncServerStatus(st *model.IngressStatus, spec *model.IngressSpec) {
	health := make(map[string]bool, len(st.Servers))
	for _, s := range st.Servers {
		health[s.Backend+"|"+s.Address] = s.Healthy
	}
	servers := make([]model.IngressServerStatus, 0)
	for _, b := range spec.Backends {
		for _, s := range b.Servers {
			servers = append(servers, model.IngressServerStatus{
				Backend: b.Name,
				Address: s.Address,
				Down:    s.Down,
				Healthy: health[b.Name+"|"+s.Address],
			})
		}
	}
	st.Servers = servers
}

// recordDeleteFailure keeps the ingress and reports why its files could not
// be removed.
func recordDeleteFailure(ing *model.Ingress, r *itemResult) {
	_, err := UpdateStatus(ing.Name, func(cur *model.Ingress) {
		cur.Status.Phase = ""
		message := "delete failed: " + r.Error
		if r.TestOutput != "" && !strings.Contains(message, r.TestOutput) {
			message += "\n" + r.TestOutput
		}
		SetCondition(&cur.Status, model.IngressCondition{
			Type:    model.IngressConditionConfigApplied,
			Status:  false,
			Reason:  r.Reason,
			Message: truncateMessage(message),
		})
	})
	if err != nil {
		logger.Errorf("Recording the delete failure of ingress %s failed: %v", ing.Name, err)
	}
	notifyWaiters(ing.Name)
}

// completeDelete drops the ingress row once its files are gone, tells the
// certificate manager and optionally purges the certificate.
func completeDelete(ctx context.Context, ing *model.Ingress, purgeCert bool) error {
	defer notifyWaiters(ing.Name)
	if err := model.UseDB().Unscoped().Where("name = ?", ing.Name).Delete(&model.Ingress{}).Error; err != nil {
		return err
	}
	certs().Forget(ing.Name)
	if purgeCert {
		if err := purgeCertificate(ing); err != nil {
			logger.Errorf("Purging the certificate of ingress %s failed: %v", ing.Name, err)
		}
	}
	return nil
}

// purgeCertificate deletes the certificate an ACME ingress issued. Its files
// are removed only when no site or stream loads them and no other
// certificate record points at them. Certificates of tls.mode cert belong to
// the user and are never purged.
func purgeCertificate(ing *model.Ingress) error {
	if ing.Spec.TLS.Mode != model.IngressTLSModeACME || ing.Status.CertID == 0 {
		return nil
	}
	db := model.UseDB()
	var cert model.Cert
	if err := db.Where("id = ?", ing.Status.CertID).First(&cert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := db.Delete(&cert).Error; err != nil {
		return err
	}

	if cert.SSLCertificatePath == "" {
		return nil
	}
	var others int64
	if err := db.Model(&model.Cert{}).
		Where("id <> ? AND ssl_certificate_path = ?", cert.ID, cert.SSLCertificatePath).
		Count(&others).Error; err != nil {
		return err
	}
	if others > 0 || len(site.BuildCertificateUsageIndex().Lookup(cert.SSLCertificatePath)) > 0 {
		return nil
	}
	for _, path := range []string{cert.SSLCertificatePath, cert.SSLCertificateKeyPath} {
		if path == "" {
			continue
		}
		if err := nginx.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
