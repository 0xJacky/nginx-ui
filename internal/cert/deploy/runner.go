package deploy

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// DefaultRetrySchedule is how long a failed push waits before each retry:
// a run makes one attempt and up to three retries.
var DefaultRetrySchedule = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute}

// keptDeployments is how many outcomes are kept per target and certificate.
const keptDeployments = 20

// Result is the outcome of one push the person asked for.
type Result struct {
	TargetID uint64 `json:"target_id"`
	CertID   uint64 `json:"cert_id"`
	// Status is model.CertDeploymentOK or model.CertDeploymentFailed.
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Runner pushes certificates to their deploy targets.
type Runner struct {
	ctx      context.Context
	schedule []time.Duration

	mu       sync.Mutex
	inflight map[runKey]*runState
	wg       sync.WaitGroup
}

type runKey struct {
	targetID uint64
	certID   uint64
}

// runState notes that a pair was triggered again while it was running.
type runState struct {
	again bool
}

// Option configures a Runner.
type Option func(*Runner)

// WithRetrySchedule replaces DefaultRetrySchedule, for tests.
func WithRetrySchedule(schedule []time.Duration) Option {
	return func(r *Runner) { r.schedule = append([]time.Duration(nil), schedule...) }
}

// NewRunner returns a runner whose retries stop when ctx ends.
func NewRunner(ctx context.Context, opts ...Option) *Runner {
	r := &Runner{ctx: ctx, schedule: DefaultRetrySchedule, inflight: map[runKey]*runState{}}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Start creates a runner and subscribes it to cert.issued and cert.renewed,
// so every enabled target bound to a certificate receives it after each
// issuance or renewal. The subscription and the retries end with ctx.
func Start(ctx context.Context) *Runner {
	r := NewRunner(ctx)
	unsubscribe := event.Subscribe(r.HandleEvent)
	go func() {
		<-ctx.Done()
		unsubscribe()
	}()
	return r
}

// HandleEvent reacts to cert.issued and cert.renewed. Bus subscribers must
// not block, so the work runs in the background.
func (r *Runner) HandleEvent(published event.Event) {
	if published.Type != event.TypeCertIssued && published.Type != event.TypeCertRenewed {
		return
	}
	certID, ok := certIDOf(published.Data)
	if !ok {
		return
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.TriggerCertificate(certID)
	}()
}

// certIDOf reads cert_id out of the event data.
func certIDOf(data any) (uint64, bool) {
	fields, ok := data.(map[string]any)
	if !ok {
		return 0, false
	}
	switch id := fields["cert_id"].(type) {
	case uint64:
		return id, id > 0
	case int:
		return uint64(id), id > 0
	case int64:
		return uint64(id), id > 0
	case float64:
		return uint64(id), id > 0
	case json.Number:
		parsed, err := id.Int64()
		return uint64(parsed), err == nil && parsed > 0
	}
	return 0, false
}

// TriggerCertificate starts a run with retries for every enabled target bound
// to a certificate.
func (r *Runner) TriggerCertificate(certID uint64) {
	targets, err := boundTargets(certID)
	if err != nil {
		logger.Errorf("Failed to load the deploy targets of certificate %d: %v", certID, err)
		return
	}
	for _, target := range targets {
		r.trigger(runKey{targetID: target.ID, certID: certID})
	}
}

// trigger starts a run for a pair, or asks the running one to go once more
// when it finishes, so a renewal during a retry is never lost.
func (r *Runner) trigger(key runKey) {
	r.mu.Lock()
	if state, running := r.inflight[key]; running {
		state.again = true
		r.mu.Unlock()
		return
	}
	state := &runState{}
	r.inflight[key] = state
	r.mu.Unlock()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			r.runWithRetries(key)

			r.mu.Lock()
			if !state.again || r.ctx.Err() != nil {
				delete(r.inflight, key)
				r.mu.Unlock()
				return
			}
			state.again = false
			r.mu.Unlock()
		}
	}()
}

// Wait blocks until every run the runner started has finished, for tests.
func (r *Runner) Wait() {
	r.wg.Wait()
}

// runWithRetries pushes a certificate to a target, retrying on failure, and
// records one outcome with the number of attempts.
func (r *Runner) runWithRetries(key runKey) {
	var (
		message string
		err     error
	)
	for attempt := 1; ; attempt++ {
		target, loadErr := query.CertDeployTarget.Where(query.CertDeployTarget.ID.Eq(key.targetID)).First()
		if loadErr != nil || !target.Enabled || !target.Matches(key.certID) {
			// The target was removed, disabled or rebound meanwhile.
			return
		}

		message, err = pushOnce(r.ctx, target, key.certID, false)
		if err == nil {
			record(key.targetID, key.certID, model.CertDeploymentOK, message, attempt)
			return
		}
		if r.ctx.Err() != nil {
			return
		}
		if attempt > len(r.schedule) {
			record(key.targetID, key.certID, model.CertDeploymentFailed, err.Error(), attempt)
			notifyFailure(target, key.certID, err)
			return
		}

		logger.Warnf("Deploying certificate %d to target %s failed (attempt %d), retrying in %s: %v",
			key.certID, target.Name, attempt, r.schedule[attempt-1], err)
		if !sleep(r.ctx, r.schedule[attempt-1]) {
			return
		}
	}
}

// sleep waits for d or until ctx ends, and reports whether it waited.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// pushOnce loads the certificate and pushes it to the target once.
func pushOnce(ctx context.Context, target *model.CertDeployTarget, certID uint64, dryRun bool) (string, error) {
	certModel, err := query.Cert.Where(query.Cert.ID.Eq(certID)).First()
	if err != nil {
		return "", err
	}
	certificate, err := LoadCertificate(certModel)
	if err != nil {
		return "", err
	}
	return Push(ctx, target.Kind, target.Config, certificate, dryRun)
}

// record stores an outcome and keeps only the latest ones of the pair.
func record(targetID, certID uint64, status, message string, attempts int) {
	q := query.CertDeployment
	deployment := &model.CertDeployment{
		TargetID: targetID,
		CertID:   certID,
		Status:   status,
		Message:  message,
		Attempts: attempts,
	}
	if err := q.Create(deployment); err != nil {
		logger.Errorf("Failed to record the deployment of certificate %d to target %d: %v", certID, targetID, err)
		return
	}

	var kept []uint64
	if err := q.Where(q.TargetID.Eq(targetID), q.CertID.Eq(certID)).
		Order(q.ID.Desc()).Limit(keptDeployments).Pluck(q.ID, &kept); err != nil || len(kept) < keptDeployments {
		return
	}
	if _, err := q.Where(q.TargetID.Eq(targetID), q.CertID.Eq(certID), q.ID.NotIn(kept...)).Delete(); err != nil {
		logger.Warnf("Failed to prune the deployments of certificate %d to target %d: %v", certID, targetID, err)
	}
}

// notifyFailure tells the person that a certificate did not reach a target
// after every retry.
func notifyFailure(target *model.CertDeployTarget, certID uint64, err error) {
	name := ""
	if certModel, loadErr := query.Cert.Where(query.Cert.ID.Eq(certID)).First(); loadErr == nil {
		name = certModel.Name
	}
	notification.Error("Deploy Certificate Error",
		"Failed to deploy certificate %{cert_name} to %{target_name}: %{error}",
		map[string]any{
			"cert_id":     certID,
			"cert_name":   name,
			"target_id":   target.ID,
			"target_name": target.Name,
			"error":       err.Error(),
		},
	)
}

// boundTargets lists the enabled targets bound to a certificate.
func boundTargets(certID uint64) ([]*model.CertDeployTarget, error) {
	q := query.CertDeployTarget
	return q.Where(q.Enabled.Is(true), q.CertID.In(0, certID)).Order(q.ID).Find()
}

// deployableCertificates lists the certificates nginx serves from files, the
// ones a target bound to every certificate receives.
func deployableCertificates() ([]*model.Cert, error) {
	q := query.Cert
	return q.Where(q.SSLCertificatePath.Neq(""), q.SSLCertificateKeyPath.Neq("")).Order(q.ID).Find()
}

// DeployTarget pushes every certificate bound to a target once, without
// retries, and records the outcomes. It runs for a disabled target too, since
// the person asked for it.
func DeployTarget(ctx context.Context, targetID uint64) ([]Result, error) {
	target, err := query.CertDeployTarget.Where(query.CertDeployTarget.ID.Eq(targetID)).First()
	if err != nil {
		return nil, err
	}

	var certIDs []uint64
	if target.CertID != 0 {
		certIDs = []uint64{target.CertID}
	} else {
		certs, err := deployableCertificates()
		if err != nil {
			return nil, err
		}
		for _, certModel := range certs {
			certIDs = append(certIDs, certModel.ID)
		}
	}

	results := make([]Result, 0, len(certIDs))
	for _, certID := range certIDs {
		results = append(results, deployNow(ctx, target, certID))
	}
	return results, nil
}

// DeployCertificate pushes a certificate to every enabled target bound to it
// once, without retries, and records the outcomes.
func DeployCertificate(ctx context.Context, certID uint64) ([]Result, error) {
	if _, err := query.Cert.Where(query.Cert.ID.Eq(certID)).First(); err != nil {
		return nil, err
	}
	targets, err := boundTargets(certID)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(targets))
	for _, target := range targets {
		results = append(results, deployNow(ctx, target, certID))
	}
	return results, nil
}

func deployNow(ctx context.Context, target *model.CertDeployTarget, certID uint64) Result {
	result := Result{TargetID: target.ID, CertID: certID, Status: model.CertDeploymentOK}
	message, err := pushOnce(ctx, target, certID, false)
	if err != nil {
		result.Status = model.CertDeploymentFailed
		message = err.Error()
	}
	result.Message = message
	record(target.ID, certID, result.Status, message, 1)
	return result
}

// DryRun checks a target configuration against a certificate without
// changing anything at the target. A certID of 0 picks the first certificate
// nginx serves from files. Nothing is recorded.
func DryRun(ctx context.Context, kind string, config map[string]string, certID uint64) (string, error) {
	if certID == 0 {
		certs, err := deployableCertificates()
		if err != nil {
			return "", err
		}
		if len(certs) == 0 {
			return "", cert.ErrNoCertificateAvailable
		}
		certID = certs[0].ID
	}
	return pushOnce(ctx, &model.CertDeployTarget{Kind: kind, Config: config}, certID, true)
}

// LatestDeployment returns the latest outcome of a target and a certificate,
// nil when there is none.
func LatestDeployment(targetID, certID uint64) *model.CertDeployment {
	q := query.CertDeployment
	deployment, err := q.Where(q.TargetID.Eq(targetID), q.CertID.Eq(certID)).Order(q.ID.Desc()).First()
	if err != nil {
		return nil
	}
	return deployment
}

// TargetsOf lists every target bound to a certificate, enabled or not.
func TargetsOf(certID uint64) ([]*model.CertDeployTarget, error) {
	q := query.CertDeployTarget
	return q.Where(q.CertID.In(0, certID)).Order(q.ID).Find()
}

// Deployments lists the latest outcomes of a target, newest first, limited to
// one certificate when certID is not 0.
func Deployments(targetID, certID uint64, limit int) ([]*model.CertDeployment, error) {
	q := query.CertDeployment
	do := q.Where(q.TargetID.Eq(targetID))
	if certID != 0 {
		do = do.Where(q.CertID.Eq(certID))
	}
	return do.Order(q.ID.Desc()).Limit(limit).Find()
}
