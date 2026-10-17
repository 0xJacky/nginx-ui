package ingress

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
)

// ReasonUnknown marks a condition nobody could determine, for example backend
// health while the availability checker is off. ComputePhase ignores it.
const ReasonUnknown = "Unknown"

// ComputePhase derives the phase from the conditions. The worker sets
// IngressPhaseApplying itself while it applies; everything else goes through
// this function.
func ComputePhase(ing *model.Ingress) string {
	status := &ing.Status
	applied, ok := FindCondition(status, model.IngressConditionConfigApplied)
	if !ok {
		return model.IngressPhasePending
	}
	if !applied.Status {
		return model.IngressPhaseError
	}
	if status.ObservedGeneration < ing.Generation {
		return model.IngressPhasePending
	}
	for _, conditionType := range []string{model.IngressConditionCertReady, model.IngressConditionBackendsHealthy} {
		c, ok := FindCondition(status, conditionType)
		if ok && !c.Status && c.Reason != ReasonUnknown {
			return model.IngressPhaseDegraded
		}
	}
	// A certificate that is served but does not match the spec yet (a host
	// added, the key type changed) is true with the reason of the reissue.
	if c, ok := FindCondition(status, model.IngressConditionCertReady); ok && c.Status && !certComplete(c.Reason) {
		return model.IngressPhaseDegraded
	}
	return model.IngressPhaseReady
}

// certComplete reports whether a true CertReady reason means the
// certificate covers the whole spec.
func certComplete(reason string) bool {
	switch reason {
	case "", CertReasonIssued, CertReasonExisting, CertReasonNotRequired:
		return true
	}
	return false
}

// statusMu serializes status writes. The apply worker, the certificate
// manager and the reconcile loop all update the same JSON column.
var statusMu sync.Mutex

// UpdateStatus loads the ingress, lets fn change its status, recomputes the
// phase unless fn set IngressPhaseApplying, and saves only the status column.
// It returns the updated ingress and wakes WaitForGeneration callers.
func UpdateStatus(name string, fn func(ing *model.Ingress)) (*model.Ingress, error) {
	defer notifyWaiters(name)
	statusMu.Lock()
	defer statusMu.Unlock()

	db := model.UseDB()
	var ing model.Ingress
	if err := db.Where("name = ?", name).First(&ing).Error; err != nil {
		return nil, err
	}
	fn(&ing)
	if ing.Status.Phase != model.IngressPhaseApplying {
		ing.Status.Phase = ComputePhase(&ing)
	}
	if err := db.Model(&ing).Select("status").Updates(&model.Ingress{Status: ing.Status}).Error; err != nil {
		return nil, err
	}
	return &ing, nil
}
