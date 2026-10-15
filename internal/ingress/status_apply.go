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
	"github.com/uozi-tech/cosy"
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
// health the reconcile loop recorded and taking down from the spec. A new
// server is not checked yet.
func syncServerStatus(st *model.IngressStatus, spec *model.IngressSpec) {
	known := make(map[string]model.IngressServerStatus, len(st.Servers))
	for _, s := range st.Servers {
		known[s.Backend+"|"+s.Address] = s
	}
	servers := make([]model.IngressServerStatus, 0)
	for _, b := range spec.Backends {
		for _, s := range b.Servers {
			previous := known[b.Name+"|"+s.Address]
			servers = append(servers, model.IngressServerStatus{
				Backend: b.Name,
				Address: s.Address,
				Down:    s.Down,
				Healthy: previous.Healthy,
				Checked: previous.Checked,
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
// certificate manager and optionally purges the certificate. A write that
// arrived after the delete was requested wins: the row is kept, the ingress
// is queued for apply so its files come back, and ErrDeleteConflict is
// returned to the delete's waiters.
func completeDelete(ctx context.Context, ing *model.Ingress, job *deleteJob) error {
	defer notifyWaiters(ing.Name)

	// storeMu orders the delete against the writes of the store: a write
	// either lands first and is seen here, or finds no row and creates one.
	storeMu.Lock()
	deleted, err := deleteRowAt(ing.Name, job)
	storeMu.Unlock()
	if err != nil {
		return err
	}
	if !deleted {
		Enqueue(ing.Name)
		return cosy.WrapErrorWithParams(ErrDeleteConflict, ing.Name)
	}

	certs().Forget(ing.Name)
	forgetApplied(ing.Name)
	if job.purgeCert {
		if err := purgeCertificate(ing); err != nil {
			logger.Errorf("Purging the certificate of ingress %s failed: %v", ing.Name, err)
		}
	}
	return nil
}

// deleteRowAt deletes the ingress row unless a write superseded the delete
// or moved the generation away from the one the delete was requested at. It
// reports false when the row is kept. A row that is already gone counts as
// deleted.
func deleteRowAt(name string, job *deleteJob) (bool, error) {
	db := model.UseDB()
	if !deleteSuperseded(job) {
		result := db.Unscoped().Where("name = ? AND generation = ?", name, job.generation).Delete(&model.Ingress{})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected > 0 {
			return true, nil
		}
	}
	var count int64
	if err := db.Model(&model.Ingress{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
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
