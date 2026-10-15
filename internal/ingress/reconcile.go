package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// Condition reasons the reconcile loop sets. The names are prefixed so they
// cannot clash with the reasons the worker and the certificate manager
// declare in their own files.
const (
	// reconcileReasonDrifted marks ConfigApplied when the files on disk no
	// longer match the rendered config. The condition stays true: the last
	// apply succeeded and nginx still runs it, the files are only queued for
	// a rewrite, after which the worker sets its own reason again.
	reconcileReasonDrifted = "Drifted"

	reconcileReasonIssued      = "Issued"
	reconcileReasonExisting    = "Existing"
	reconcileReasonNotRequired = "NotRequired"
	reconcileReasonCertMissing = "CertNotFound"
	reconcileReasonExpired     = "Expired"

	reconcileReasonAllHealthy       = "AllHealthy"
	reconcileReasonPartiallyHealthy = "PartiallyHealthy"
	reconcileReasonNoHealthyServer  = "NoHealthyServer"
)

// reconcileFirstDelay is how long after start the first pass runs, so it does
// not compete with the worker applying what queued up while Nginx UI was down.
const reconcileFirstDelay = time.Minute

// Seams for tests. Render, Enqueue and the certificate manager are built in
// other files of this package; the availability lookup reads the global
// upstream checker.
var (
	reconcileRender      = Render
	reconcileEnqueue     = Enqueue
	reconcileResolveCert = func(ctx context.Context, ing *model.Ingress) (CertState, error) {
		return certs().Resolve(ctx, ing)
	}
	reconcileEnsureCert = func(ctx context.Context, ing *model.Ingress) {
		certs().Ensure(ctx, ing)
	}
	// reconcileAvailability returns the checker results keyed by socket
	// address, and false when the checker is turned off, in which case the
	// results are stale or missing.
	reconcileAvailability = func() (map[string]*upstream.Status, bool) {
		if !settings.UpstreamCheckSettings.Enabled {
			return nil, false
		}
		return upstream.GetUpstreamService().GetAvailabilityMap(), true
	}
	reconcileNow = time.Now
)

func startReconcile(ctx context.Context) {
	timer := time.NewTimer(reconcileFirstDelay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		if err := ReconcileAll(ctx); err != nil {
			logger.Warnf("Ingress reconcile pass finished with errors: %v", err)
		}
		// Read the interval every pass so a settings change applies without
		// a restart.
		timer.Reset(settings.IngressSettings.GetReconcileInterval())
	}
}

// ReconcileAll compares every ingress with its files on disk, enqueues the
// ones that drifted or are missing, and refreshes certificate and backend
// health in their status. A failure on one ingress does not stop the pass;
// the failures are returned joined.
func ReconcileAll(ctx context.Context) error {
	var items []*model.Ingress
	if err := model.UseDB().Order("name").Find(&items).Error; err != nil {
		return err
	}

	var errs []error
	for _, ing := range items {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := reconcileIngress(ctx, ing); err != nil {
			logger.Errorf("Ingress reconcile failed: ingress=%q error=%v", ing.Name, err)
			errs = append(errs, fmt.Errorf("%s: %w", ing.Name, err))
		}
	}
	return errors.Join(errs...)
}

// reconcileIngress handles one ingress. A panic is turned into an error so
// it cannot end the pass for the others.
func reconcileIngress(ctx context.Context, ing *model.Ingress) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	// The worker applies a newer generation anyway, and the files on disk
	// belong to the previous one until it does.
	if ing.Status.ObservedGeneration < ing.Generation {
		return nil
	}

	var errs []error
	plan := reconcilePlan{
		generation:         ing.Generation,
		observedGeneration: ing.Status.ObservedGeneration,
	}

	state, certErr := reconcileResolveCert(ctx, ing)
	if certErr != nil {
		errs = append(errs, fmt.Errorf("resolve certificate: %w", certErr))
	} else {
		plan.cert = &state
		plan.certChanged = certStatusChanged(ing, state, reconcileNow())
		rendered, renderErr := reconcileRender(ing, RenderOptions{Cert: state})
		if renderErr != nil {
			errs = append(errs, fmt.Errorf("render: %w", renderErr))
		} else {
			drifted, driftErr := driftedFiles(ing, rendered)
			if driftErr != nil {
				errs = append(errs, fmt.Errorf("compare files: %w", driftErr))
			}
			plan.drifted = drifted
		}
	}

	avail, checkerEnabled := reconcileAvailability()
	plan.servers, plan.health = backendHealth(&ing.Spec, avail, checkerEnabled)

	plan.decide(ing)

	if plan.statusChanges(ing) {
		if _, err := UpdateStatus(ing.Name, plan.apply); err != nil {
			errs = append(errs, fmt.Errorf("update status: %w", err))
		}
	}
	if plan.enqueue {
		reconcileEnqueue(ing.Name)
	}
	// Issuance only runs in memory: after a restart, or once a retry timer
	// was lost, nothing would start it again. Ensure is idempotent and keeps
	// its backoff, so asking every pass is cheap. It runs after the status
	// write so its own CertReady progress is not overwritten.
	if plan.cert != nil && needsIssuance(ing, *plan.cert) {
		reconcileEnsureCert(ctx, ing)
	}
	return errors.Join(errs...)
}

// needsIssuance reports whether an ACME ingress lacks a certificate that
// covers every host: none is ready, or the ready one is served while a
// reissue is due (its reason is not Issued).
func needsIssuance(ing *model.Ingress, state CertState) bool {
	if ing.Spec.TLS.Mode != model.IngressTLSModeACME {
		return false
	}
	return !state.Ready || (state.Reason != "" && state.Reason != CertReasonIssued)
}

// reconcilePlan holds what one pass found for an ingress and turns it into a
// status change. apply runs on the row UpdateStatus loads, which may be newer
// than the one the pass read.
type reconcilePlan struct {
	generation         int64
	observedGeneration int64

	cert        *CertState
	certChanged bool
	drifted     []string

	servers []model.IngressServerStatus
	health  model.IngressCondition

	markDrifted bool
	enqueue     bool
}

// decide picks the ConfigApplied update and whether to enqueue.
func (p *reconcilePlan) decide(ing *model.Ingress) {
	applied, hasApplied := FindCondition(&ing.Status, model.IngressConditionConfigApplied)
	applying := ing.Status.Phase == model.IngressPhaseApplying

	switch {
	case applying:
		// The worker is writing the files right now.
	case hasApplied && !applied.Status:
		// The last apply failed and its reason must stay visible. Retrying
		// only helps when the certificate changed under it.
		p.enqueue = p.certChanged
	case len(p.drifted) > 0:
		p.enqueue = true
		// A new certificate changes the desired files, the disk did not
		// drift: just apply.
		p.markDrifted = !p.certChanged
		if p.markDrifted {
			logger.Warnf("Ingress files drifted from the spec, queued for rewrite: ingress=%q files=%s",
				ing.Name, strings.Join(p.drifted, ", "))
		} else {
			logger.Infof("Ingress certificate changed, queued for apply: ingress=%q", ing.Name)
		}
	}
}

// apply writes the findings into the status of cur.
func (p *reconcilePlan) apply(cur *model.Ingress) {
	cur.Status.Servers = p.servers
	SetCondition(&cur.Status, p.health)

	// Everything else was derived from a generation the row may have left
	// behind meanwhile.
	if cur.Generation != p.generation || cur.Status.ObservedGeneration != p.observedGeneration {
		return
	}

	if p.cert != nil {
		applyCertStatus(cur, *p.cert, reconcileNow())
	}
	if p.markDrifted && cur.Status.Phase != model.IngressPhaseApplying {
		if c, ok := FindCondition(&cur.Status, model.IngressConditionConfigApplied); ok && c.Status {
			SetCondition(&cur.Status, model.IngressCondition{
				Type:    model.IngressConditionConfigApplied,
				Status:  true,
				Reason:  reconcileReasonDrifted,
				Message: "files differ from the rendered config and are queued for rewrite: " + strings.Join(p.drifted, ", "),
			})
		}
	}
}

// statusChanges reports whether apply would change the stored status, so an
// unchanged ingress costs no write.
func (p *reconcilePlan) statusChanges(ing *model.Ingress) bool {
	before, err := json.Marshal(ing.Status)
	if err != nil {
		return true
	}
	probe := *ing
	if err := json.Unmarshal(before, &probe.Status); err != nil {
		return true
	}
	p.apply(&probe)
	if probe.Status.Phase != model.IngressPhaseApplying {
		probe.Status.Phase = ComputePhase(&probe)
	}
	after, err := json.Marshal(probe.Status)
	if err != nil {
		return true
	}
	return !bytes.Equal(before, after)
}

// driftedFiles lists the generated files that are missing or differ from the
// rendered output, and a missing sites-enabled link. Upstream files of
// backends that were removed are the apply worker's to clean up.
func driftedFiles(ing *model.Ingress, rendered *Rendered) ([]string, error) {
	if rendered == nil {
		return nil, errors.New("render returned nothing")
	}

	var drifted []string
	var errs []error

	sitePath, err := site.ResolveAvailablePath(rendered.SiteName)
	if err != nil {
		return nil, err
	}
	if state, err := compareFile(sitePath, rendered.SiteContent); err != nil {
		errs = append(errs, err)
	} else if state != "" {
		drifted = append(drifted, "sites-available/"+rendered.SiteName+" "+state)
	}

	// A remote-deploy namespace never gets a local link.
	if !site.ResolveNamespaceByID(ing.NamespaceID).IsRemoteDeploy() {
		enabledPath, err := site.ResolveEnabledPath(rendered.SiteName)
		if err != nil {
			errs = append(errs, err)
		} else if exists, err := nginx.Exists(nginx.GetConfSymlinkPath(enabledPath)); err != nil {
			errs = append(errs, err)
		} else if !exists {
			drifted = append(drifted, "sites-enabled/"+rendered.SiteName+" missing")
		}
	}

	names := make([]string, 0, len(rendered.Upstreams))
	for name := range rendered.Upstreams {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path, err := managed.Path(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		state, err := compareFile(path, rendered.Upstreams[name])
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if state != "" {
			drifted = append(drifted, "conf.d/"+managed.FileName(name)+" "+state)
		}
	}

	return drifted, errors.Join(errs...)
}

// compareFile returns "" when the file holds exactly want, "missing" or
// "modified" otherwise.
func compareFile(path, want string) (string, error) {
	got, err := nginx.ReadFile(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if !bytes.Equal(got, []byte(want)) {
		return "modified", nil
	}
	return "", nil
}

// certCondition is the CertReady condition for a resolved certificate. When
// the certificate is not ready and the manager gave no reason, a false
// condition already in the status (e.g. Issuing, set by the manager) is kept.
func certCondition(ing *model.Ingress, state CertState, now time.Time) model.IngressCondition {
	c := model.IngressCondition{Type: model.IngressConditionCertReady}
	switch {
	case ing.Spec.TLS.Mode == model.IngressTLSModeNone || ing.Spec.TLS.Mode == "":
		c.Status, c.Reason = true, reconcileReasonNotRequired
	case state.Ready && state.NotAfter != nil && !state.NotAfter.After(now):
		c.Reason = reconcileReasonExpired
		c.Message = "certificate expired at " + state.NotAfter.UTC().Format(time.RFC3339)
	case state.Ready:
		// The manager may report a ready certificate with its own reason,
		// e.g. Issuing while a reissue for an added host runs and the
		// existing hosts keep their listener.
		c.Status, c.Reason, c.Message = true, state.Reason, state.Message
		if c.Reason == "" {
			c.Reason = reconcileReasonIssued
			if ing.Spec.TLS.Mode == model.IngressTLSModeCert {
				c.Reason = reconcileReasonExisting
			}
		}
	default:
		c.Reason, c.Message = state.Reason, state.Message
		if c.Reason == "" {
			if have, ok := FindCondition(&ing.Status, model.IngressConditionCertReady); ok && !have.Status {
				return have
			}
			c.Reason = reconcileReasonCertMissing
		}
	}
	return c
}

// certStatusChanged reports whether the resolved certificate differs from what
// the status records: readiness, reason, certificate or expiry.
func certStatusChanged(ing *model.Ingress, state CertState, now time.Time) bool {
	want := certCondition(ing, state, now)
	have, ok := FindCondition(&ing.Status, model.IngressConditionCertReady)
	if !ok || have.Status != want.Status || have.Reason != want.Reason {
		return true
	}
	if ing.Spec.TLS.Mode == model.IngressTLSModeNone {
		return false
	}
	return ing.Status.CertID != state.CertID || !sameTime(ing.Status.CertNotAfter, state.NotAfter)
}

func applyCertStatus(cur *model.Ingress, state CertState, now time.Time) {
	want := certCondition(cur, state, now)
	// Keep the message of the component that set the condition when only the
	// expiry moved, e.g. after a renewal.
	if have, ok := FindCondition(&cur.Status, model.IngressConditionCertReady); ok &&
		have.Status == want.Status && have.Reason == want.Reason && want.Message == "" {
		want.Message = have.Message
	}
	SetCondition(&cur.Status, want)
	if cur.Spec.TLS.Mode == model.IngressTLSModeNone {
		cur.Status.CertID = 0
		cur.Status.CertNotAfter = nil
		return
	}
	cur.Status.CertID = state.CertID
	cur.Status.CertNotAfter = state.NotAfter
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// backendHealth maps every spec server to the availability checker results
// and derives the BackendsHealthy condition:
//   - NoHealthyServer (false): a backend has no server that is both not down
//     and healthy, as far as the checker knows;
//   - Unknown (false, ignored by the phase): the checker is off or has no
//     result yet for a backend without a healthy server;
//   - PartiallyHealthy (true): every backend has a healthy server, some
//     servers that are not down are unhealthy or not checked yet;
//   - AllHealthy (true): every server that is not down is healthy.
//
// Servers.Healthy is the raw probe result, also for servers marked down, so
// a controller can see whether a drained server is back before undraining it.
// Servers.Checked tells a server the checker has no result for (yet) from an
// unhealthy one.
func backendHealth(spec *model.IngressSpec, avail map[string]*upstream.Status, checkerEnabled bool) ([]model.IngressServerStatus, model.IngressCondition) {
	servers := make([]model.IngressServerStatus, 0)
	var noHealthy, unknown, partial []string

	for _, backend := range spec.Backends {
		usable, healthy, unchecked := 0, 0, 0
		for _, server := range backend.Servers {
			status, checked := lookupAvailability(avail, server.Address)
			checked = checked && checkerEnabled
			isHealthy := checked && status.Online
			servers = append(servers, model.IngressServerStatus{
				Backend: backend.Name,
				Address: server.Address,
				Down:    server.Down,
				Healthy: isHealthy,
				Checked: checked,
			})
			if server.Down {
				continue
			}
			usable++
			switch {
			case !checked:
				unchecked++
			case isHealthy:
				healthy++
			}
		}

		switch {
		case usable == 0, healthy == 0 && unchecked == 0:
			noHealthy = append(noHealthy, backend.Name)
		case healthy == 0:
			unknown = append(unknown, backend.Name)
		case healthy < usable:
			partial = append(partial, backend.Name)
		}
	}

	c := model.IngressCondition{Type: model.IngressConditionBackendsHealthy}
	switch {
	case len(noHealthy) > 0:
		c.Reason = reconcileReasonNoHealthyServer
		c.Message = "no healthy server that is not down in backend " + strings.Join(noHealthy, ", ")
	case !checkerEnabled:
		c.Reason = ReasonUnknown
		c.Message = "the upstream availability check is disabled"
	case len(unknown) > 0:
		c.Reason = ReasonUnknown
		c.Message = "no availability result yet for backend " + strings.Join(unknown, ", ")
	case len(partial) > 0:
		c.Status = true
		c.Reason = reconcileReasonPartiallyHealthy
		c.Message = "some servers are unhealthy or not checked yet in backend " + strings.Join(partial, ", ")
	default:
		c.Status = true
		c.Reason = reconcileReasonAllHealthy
	}
	return servers, c
}

// refreshHealthAfterApply refreshes the backend health of freshly applied
// ingresses in the background, so a new or changed ingress does not wait for
// the next reconcile pass. A variable so tests can observe it.
var refreshHealthAfterApply = func(names []string) {
	go refreshBackendHealth(names)
}

// refreshBackendHealth runs an availability check (the apply registered the
// new servers with the checker) and records the backend health of the named
// ingresses.
func refreshBackendHealth(names []string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Refreshing ingress backend health panicked: %v", r)
		}
	}()
	if settings.UpstreamCheckSettings.Enabled {
		upstream.GetUpstreamService().PerformAvailabilityTest()
	}
	for _, name := range names {
		if err := recordBackendHealth(name); err != nil {
			logger.Errorf("Recording the backend health of ingress %s failed: %v", name, err)
		}
	}
}

// recordBackendHealth writes the current checker results into the status of
// one ingress.
func recordBackendHealth(name string) error {
	avail, checkerEnabled := reconcileAvailability()
	_, err := UpdateStatus(name, func(cur *model.Ingress) {
		servers, health := backendHealth(&cur.Spec, avail, checkerEnabled)
		cur.Status.Servers = servers
		SetCondition(&cur.Status, health)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func lookupAvailability(avail map[string]*upstream.Status, address string) (*upstream.Status, bool) {
	key := upstream.SocketAddress(address)
	if key == "" {
		return nil, false
	}
	status, ok := avail[key]
	if !ok || status == nil {
		return nil, false
	}
	return status, true
}
