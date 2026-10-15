package ingress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// Reasons of the CertReady condition.
const (
	CertReasonIssued       = "Issued"
	CertReasonExisting     = "Existing"
	CertReasonNotRequired  = "NotRequired"
	CertReasonIssuing      = "Issuing"
	CertReasonDNSNotReady  = "DNSNotReady"
	CertReasonIssueFailed  = "IssueFailed"
	CertReasonCertNotFound = "CertNotFound"
	CertReasonExpired      = "Expired"
)

// Retry backoff after a failed DNS check or issuance: 1m, 2m, 4m, ... 30m.
const (
	certBackoffMin = time.Minute
	certBackoffMax = 30 * time.Minute
	// certRouteProbeTimeout bounds the loopback HTTP-01 route probe.
	certRouteProbeTimeout = 30 * time.Second
)

// newCertManager returns the certificate manager Start installs.
func newCertManager() CertManager {
	return newACMECertManager()
}

// certRequest is what one issuance run works on: a snapshot of the ingress
// taken when the run started.
type certRequest struct {
	// Name is the ingress name.
	Name string
	// Hosts are the ingress hosts in ASCII (punycode) form.
	Hosts []string
	TLS   model.IngressTLS
}

// SiteName is the generated site name, which is also the cert record's
// Filename.
func (r certRequest) SiteName() string {
	return SiteName(r.Name)
}

// KeyType is the normalized key type, RSA 2048 by default.
func (r certRequest) KeyType() certcrypto.KeyType {
	return helper.GetKeyType(certcrypto.KeyType(strings.TrimSpace(r.TLS.KeyType)))
}

// Challenge is the challenge method, HTTP-01 by default.
func (r certRequest) Challenge() string {
	if r.TLS.Challenge == model.IngressChallengeDNS01 {
		return cert.DNS01
	}
	return cert.HTTP01
}

// fingerprint changes whenever a spec change could make a failed issuance
// succeed, which resets the backoff.
func (r certRequest) fingerprint() string {
	return strings.Join([]string{
		strings.Join(r.Hosts, ","),
		string(r.KeyType()),
		r.Challenge(),
		strconv.FormatUint(r.TLS.DNSCredentialID, 10),
		strconv.FormatUint(r.TLS.ACMEUserID, 10),
	}, "|")
}

// payload builds the issuance payload. ConfigName stays empty: the manager
// probes the challenge route itself before issuing.
func (r certRequest) payload() *cert.ConfigPayload {
	return &cert.ConfigPayload{
		ServerName:      append([]string(nil), r.Hosts...),
		ChallengeMethod: r.Challenge(),
		DNSCredentialID: r.TLS.DNSCredentialID,
		ACMEUserID:      r.TLS.ACMEUserID,
		KeyType:         r.KeyType(),
	}
}

func newCertRequest(ing *model.Ingress) certRequest {
	return certRequest{Name: ing.Name, Hosts: asciiHosts(ing.Spec.Hosts), TLS: ing.Spec.TLS}
}

// asciiHosts converts the hosts to their punycode form, dropping blanks and
// duplicates.
func asciiHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	for _, host := range hosts {
		name := dns.ToASCIIName(host)
		if name == "" || slices.Contains(out, name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// certIssuer issues the certificate of payload for the site name and returns
// the cert record.
type certIssuer func(ctx context.Context, siteName string, payload *cert.ConfigPayload) (*model.Cert, error)

// certProber checks that the hosts of req can pass an HTTP-01 validation. It
// returns the DNS evidence it collected, used to explain a later issuance
// failure, and a *certProbeError when issuance should not be attempted.
type certProber func(ctx context.Context, req certRequest) (acmehint.Evidence, error)

// certProbeError stops an issuance before the CA is contacted.
type certProbeError struct {
	// Reason is the CertReady reason to report.
	Reason string
	Err    error
}

func (e *certProbeError) Error() string { return e.Err.Error() }

func (e *certProbeError) Unwrap() error { return e.Err }

// acmeState is the in-memory issuance state of one ACME ingress.
type acmeState struct {
	// token identifies the run that owns the state; results of older runs
	// (or runs of a forgotten ingress) are dropped.
	token       uint64
	fingerprint string
	inFlight    bool
	cancel      context.CancelFunc
	// reason and message describe the current or last run.
	reason  string
	message string
	// failures counts the consecutive failed runs; retryAt is when the next
	// one may start.
	failures  int
	retryAt   time.Time
	stopRetry func() bool
}

// acmeCertManager resolves existing certificates and issues ACME ones in the
// background. Every external effect is a field so tests run without ACME,
// DNS or nginx.
type acmeCertManager struct {
	issue certIssuer
	probe certProber
	now   func() time.Time
	// afterFunc runs f after d and returns a function that cancels it.
	afterFunc    func(d time.Duration, f func()) (stop func() bool)
	enqueue      func(names ...string)
	updateStatus func(name string, fn func(ing *model.Ingress)) (*model.Ingress, error)
	load         func(name string) (*model.Ingress, error)

	// sem allows one issuance at a time across all ingresses, keeping the
	// ACME rate limits and the challenge port out of contention.
	sem chan struct{}

	mu     sync.Mutex
	seq    uint64
	states map[string]*acmeState
}

func newACMECertManager() *acmeCertManager {
	return &acmeCertManager{
		issue: func(ctx context.Context, siteName string, payload *cert.ConfigPayload) (*model.Cert, error) {
			return site.IssueCertificate(ctx, siteName, payload, nil)
		},
		probe: probeCertHosts,
		now:   time.Now,
		afterFunc: func(d time.Duration, f func()) func() bool {
			return time.AfterFunc(d, f).Stop
		},
		enqueue:      Enqueue,
		updateStatus: UpdateStatus,
		load:         loadIngress,
		sem:          make(chan struct{}, 1),
		states:       map[string]*acmeState{},
	}
}

func loadIngress(name string) (*model.Ingress, error) {
	var ing model.Ingress
	if err := model.UseDB().Where("name = ?", name).First(&ing).Error; err != nil {
		return nil, err
	}
	return &ing, nil
}

// --- Resolve ---

// Resolve returns the certificate the ingress can serve now:
//   - none: not ready, NotRequired;
//   - cert: the certificate cert_id when its files load, it has not expired
//     and it covers every host (Existing); CertNotFound or Expired otherwise;
//   - acme: the cert record of the generated site when it is valid and
//     covers every host (Issued). Otherwise the issuance state (Issuing,
//     DNSNotReady, IssueFailed). A valid certificate that misses hosts added
//     later is still served (Ready with the issuance reason) so the existing
//     hosts keep their HTTPS listener while the new one is issued.
//
// It reads the database and the certificate files but changes nothing.
func (m *acmeCertManager) Resolve(_ context.Context, ing *model.Ingress) (CertState, error) {
	switch ing.Spec.TLS.Mode {
	case model.IngressTLSModeNone, "":
		return CertState{Reason: CertReasonNotRequired}, nil
	case model.IngressTLSModeCert:
		return m.resolveExisting(ing)
	case model.IngressTLSModeACME:
		return m.resolveACME(newCertRequest(ing))
	default:
		return CertState{}, cosy.WrapErrorWithParams(ErrInvalidTLS, fmt.Sprintf("unknown mode %q", ing.Spec.TLS.Mode))
	}
}

func (m *acmeCertManager) resolveExisting(ing *model.Ingress) (CertState, error) {
	id := ing.Spec.TLS.CertID
	if id == 0 {
		return CertState{Reason: CertReasonCertNotFound, Message: "tls.cert_id is not set"}, nil
	}
	var row model.Cert
	err := model.UseDB().Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CertState{Reason: CertReasonCertNotFound, Message: fmt.Sprintf("certificate %d not found", id)}, nil
	}
	if err != nil {
		return CertState{}, err
	}

	check := checkCertificate(&row, asciiHosts(ing.Spec.Hosts), m.now())
	state := check.state()
	switch {
	case check.problem != "":
		state.Reason = CertReasonCertNotFound
		state.Message = check.problem
	case check.expired:
		state.Reason = CertReasonExpired
		state.Message = fmt.Sprintf("certificate %s expired at %s", check.name, check.notAfter.UTC().Format(time.RFC3339))
	case len(check.uncovered) > 0:
		state.Reason = CertReasonCertNotFound
		state.Message = fmt.Sprintf("certificate %s does not cover %s", check.name, strings.Join(check.uncovered, ", "))
	default:
		state.Ready = true
		state.Reason = CertReasonExisting
	}
	return state, nil
}

func (m *acmeCertManager) resolveACME(req certRequest) (CertState, error) {
	row, err := findACMECert(req)
	if err != nil {
		return CertState{}, err
	}

	var check certCheck
	if row != nil {
		check = checkCertificate(row, req.Hosts, m.now())
		if check.complete() {
			state := check.state()
			state.Ready = true
			state.Reason = CertReasonIssued
			return state, nil
		}
	}

	reason, message := m.issuanceState(req.Name, req.fingerprint())
	if reason == "" {
		// Nothing recorded in memory, e.g. after a restart: report the last
		// failure of the record, else issuance that has yet to start.
		reason = CertReasonIssuing
		if row != nil && row.Status == model.CertStatusFailure && row.LastError != "" {
			reason, message = CertReasonIssueFailed, row.LastError
		}
	}

	if check.usable() {
		// Keep serving the current certificate for the hosts it covers.
		state := check.state()
		state.Ready = true
		state.Reason = reason
		state.Message = joinMessages(
			fmt.Sprintf("the certificate does not cover %s yet", strings.Join(check.uncovered, ", ")), message)
		return state, nil
	}

	state := CertState{Reason: reason, Message: message}
	if row != nil && check.expired {
		state.Message = joinMessages(
			fmt.Sprintf("the certificate expired at %s", check.notAfter.UTC().Format(time.RFC3339)), message)
	}
	return state, nil
}

// issuanceState returns the reason and message recorded for the ingress, if
// they belong to the given request.
func (m *acmeCertManager) issuanceState(name, fingerprint string) (reason, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[name]
	if s == nil || (s.fingerprint != fingerprint && !s.inFlight) {
		return "", ""
	}
	return s.reason, s.message
}

// findACMECert returns the cert record of the generated site for the key type
// of the request, or nil.
func findACMECert(req certRequest) (*model.Cert, error) {
	var rows []model.Cert
	err := model.UseDB().
		Where("filename = ? AND key_type IN ?", req.SiteName(), helper.GetKeyTypeAliasStrings(req.KeyType())).
		Order("id").Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// certCheck is the outcome of checking a cert record against the hosts.
type certCheck struct {
	id       uint64
	name     string
	certPath string
	keyPath  string
	notAfter *time.Time
	// problem explains why the files cannot be served.
	problem   string
	expired   bool
	uncovered []string
}

func checkCertificate(row *model.Cert, hosts []string, now time.Time) certCheck {
	check := certCheck{
		id:       row.ID,
		name:     cert.CertificateName(fmt.Sprintf("#%d", row.ID), row.Domains),
		certPath: strings.TrimSpace(row.SSLCertificatePath),
		keyPath:  strings.TrimSpace(row.SSLCertificateKeyPath),
	}
	if strings.TrimSpace(row.Name) != "" {
		check.name = strings.TrimSpace(row.Name)
	}
	if check.certPath == "" || check.keyPath == "" {
		check.problem = fmt.Sprintf("certificate %s has no certificate or key file", check.name)
		return check
	}
	pair, err := cert.LoadCertificatePair(check.certPath, check.keyPath)
	if err != nil {
		check.problem = fmt.Sprintf("certificate %s cannot be loaded: %v", check.name, err)
		return check
	}
	notAfter := pair.Leaf.NotAfter
	check.notAfter = &notAfter
	check.expired = !now.Before(notAfter)
	for _, host := range hosts {
		if !cert.CertificateCoversIdentifier(pair.Leaf, host) {
			check.uncovered = append(check.uncovered, host)
		}
	}
	return check
}

// usable reports whether the files load and have not expired.
func (c certCheck) usable() bool {
	return c.problem == "" && c.notAfter != nil && !c.expired
}

// complete reports whether the certificate is usable for every host.
func (c certCheck) complete() bool {
	return c.usable() && len(c.uncovered) == 0
}

// state returns the certificate facts; Ready and Reason are left to the caller.
func (c certCheck) state() CertState {
	if c.problem != "" {
		return CertState{CertID: c.id}
	}
	return CertState{CertID: c.id, CertPath: c.certPath, KeyPath: c.keyPath, NotAfter: c.notAfter}
}

func joinMessages(parts ...string) string {
	var out []string
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ": ")
}

// --- Ensure ---

// Ensure starts a background issuance for an ACME ingress whose certificate
// is missing, expired or misses a host, unless one is already running for it
// or the backoff after the last failure has not elapsed. A changed host list,
// key type, challenge, DNS credential or ACME user resets the backoff.
func (m *acmeCertManager) Ensure(_ context.Context, ing *model.Ingress) {
	if ing == nil {
		return
	}
	if ing.Spec.TLS.Mode != model.IngressTLSModeACME {
		m.Forget(ing.Name)
		return
	}
	req := newCertRequest(ing)
	if len(req.Hosts) == 0 {
		return
	}

	row, err := findACMECert(req)
	if err != nil {
		logger.Error("ingress: look up certificate of", ing.Name, err)
		return
	}
	complete := row != nil && checkCertificate(row, req.Hosts, m.now()).complete()

	fingerprint := req.fingerprint()
	m.mu.Lock()
	s := m.states[req.Name]
	switch {
	case complete:
		if s != nil && !s.inFlight {
			m.dropLocked(req.Name)
		}
		m.mu.Unlock()
		return
	case s != nil && s.inFlight:
		m.mu.Unlock()
		return
	case s != nil && s.fingerprint == fingerprint && m.now().Before(s.retryAt):
		m.mu.Unlock()
		return
	}
	if s == nil || s.fingerprint != fingerprint {
		m.dropLocked(req.Name)
		s = &acmeState{fingerprint: fingerprint}
		m.states[req.Name] = s
	}
	if s.stopRetry != nil {
		s.stopRetry()
		s.stopRetry = nil
	}
	m.seq++
	token := m.seq
	ctx, cancel := context.WithCancel(context.Background())
	s.token = token
	s.inFlight = true
	s.cancel = cancel
	s.reason = CertReasonIssuing
	s.message = ""
	m.mu.Unlock()

	m.writeCondition(req.Name)
	go m.run(ctx, token, req)
}

// run probes and issues one certificate. It never blocks the caller of Ensure.
func (m *acmeCertManager) run(ctx context.Context, token uint64, req certRequest) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("ingress: certificate issuance for %s panicked: %v\n%s", req.Name, r, debug.Stack())
			m.finish(token, req.Name, CertReasonIssueFailed, fmt.Sprintf("issuance panicked: %v", r))
		}
	}()

	select {
	case m.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-m.sem }()
	if ctx.Err() != nil {
		return
	}

	var evidence acmehint.Evidence
	if req.Challenge() == cert.HTTP01 {
		var err error
		evidence, err = m.probe(ctx, req)
		if err != nil {
			reason := CertReasonDNSNotReady
			var probeErr *certProbeError
			if errors.As(err, &probeErr) && probeErr.Reason != "" {
				reason = probeErr.Reason
			}
			m.finish(token, req.Name, reason, err.Error())
			return
		}
	}
	if ctx.Err() != nil {
		return
	}

	payload := req.payload()
	certModel, err := m.issue(ctx, req.SiteName(), payload)
	if err != nil {
		message := cert.ShortError(err)
		if hint := acmehint.Classify(err, evidence); hint != nil && hint.Code != acmehint.CodeUnknown {
			message = joinMessages(message, hint.Message)
		}
		m.finish(token, req.Name, CertReasonIssueFailed, message)
		return
	}

	if err := keepCertRenewed(certModel, payload); err != nil {
		logger.Error("ingress: enable auto renewal of", req.SiteName(), err)
	}
	// Verify what was issued so a record that still does not serve the hosts
	// backs off instead of being issued again right away.
	row, err := findACMECert(req)
	if err != nil {
		m.finish(token, req.Name, CertReasonIssueFailed, err.Error())
		return
	}
	if row == nil {
		m.finish(token, req.Name, CertReasonIssueFailed, "the issued certificate record was not found")
		return
	}
	if check := checkCertificate(row, req.Hosts, m.now()); !check.complete() {
		message := check.problem
		if message == "" {
			message = fmt.Sprintf("the issued certificate does not cover %s", strings.Join(check.uncovered, ", "))
		}
		m.finish(token, req.Name, CertReasonIssueFailed, message)
		return
	}
	m.finish(token, req.Name, "", "")
}

// finish records the outcome of a run. An empty reason is a success: the
// state is dropped and the ingress is enqueued so the worker renders the
// HTTPS listener. A failure backs off and schedules the retry. Outcomes of
// runs that no longer own the state are dropped.
func (m *acmeCertManager) finish(token uint64, name, reason, message string) {
	m.mu.Lock()
	s := m.states[name]
	if s == nil || s.token != token {
		m.mu.Unlock()
		return
	}
	s.inFlight = false
	s.cancel = nil
	if reason == "" {
		delete(m.states, name)
		m.mu.Unlock()
		m.enqueue(name)
		return
	}

	s.failures++
	delay := certBackoff(s.failures)
	s.reason = reason
	s.message = message
	s.retryAt = m.now().Add(delay)
	s.stopRetry = m.afterFunc(delay, func() { m.retry(name, token) })
	m.mu.Unlock()

	logger.Warnf("ingress: certificate for %s not issued (%s): %s; retrying in %s", name, reason, message, delay)
	m.writeCondition(name)
}

// retry runs Ensure again once the backoff of a failed run has elapsed.
func (m *acmeCertManager) retry(name string, token uint64) {
	m.mu.Lock()
	s := m.states[name]
	current := s != nil && s.token == token && !s.inFlight
	m.mu.Unlock()
	if !current {
		return
	}
	ing, err := m.load(name)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("ingress: load", name, "for certificate retry:", err)
		}
		return
	}
	m.Ensure(context.Background(), ing)
}

// certBackoff returns the wait after the given number of consecutive failures.
func certBackoff(failures int) time.Duration {
	delay := certBackoffMin
	for i := 1; i < failures && delay < certBackoffMax; i++ {
		delay *= 2
	}
	return min(delay, certBackoffMax)
}

// writeCondition stores the CertReady condition as Resolve reports it, so the
// issuance progress is visible between applies.
func (m *acmeCertManager) writeCondition(name string) {
	_, err := m.updateStatus(name, func(ing *model.Ingress) {
		if ing.Spec.TLS.Mode != model.IngressTLSModeACME {
			return
		}
		state, err := m.Resolve(context.Background(), ing)
		if err != nil {
			logger.Error("ingress: resolve certificate of", name, err)
			return
		}
		SetCondition(&ing.Status, model.IngressCondition{
			Type:    model.IngressConditionCertReady,
			Status:  state.Ready,
			Reason:  state.Reason,
			Message: state.Message,
		})
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("ingress: update certificate status of", name, err)
	}
}

// --- Forget ---

// Forget cancels a pending run and drops the state and backoff of an ingress.
// A run already talking to the CA completes, but its outcome is ignored.
func (m *acmeCertManager) Forget(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropLocked(name)
}

func (m *acmeCertManager) dropLocked(name string) {
	s := m.states[name]
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.stopRetry != nil {
		s.stopRetry()
	}
	delete(m.states, name)
}

// --- effects ---

// keepCertRenewed makes sure the auto certificate job renews the record and
// that it points at the issued files.
func keepCertRenewed(certModel *model.Cert, payload *cert.ConfigPayload) error {
	if certModel == nil {
		return errors.New("no certificate record")
	}
	db := model.UseDB()
	var row model.Cert
	if err := db.Where("id = ?", certModel.ID).First(&row).Error; err != nil {
		return err
	}
	updates := map[string]any{}
	if row.AutoCert != model.AutoCertEnabled {
		updates["auto_cert"] = model.AutoCertEnabled
	}
	if strings.TrimSpace(row.SSLCertificatePath) == "" {
		updates["ssl_certificate_path"] = payload.GetCertificatePath()
	}
	if strings.TrimSpace(row.SSLCertificateKeyPath) == "" {
		updates["ssl_certificate_key_path"] = payload.GetCertificateKeyPath()
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&model.Cert{}).Where("id = ?", row.ID).Updates(updates).Error
}

// Seams of probeCertHosts for tests.
var (
	diagnoseCertHosts = acmehint.Diagnose
	probeHTTP01Routes = cert.ProbeHTTP01Routes
	localAddrs        = interfaceAddrs
	certCADirOf       = certCADir
)

// probeCertHosts checks, before the CA is asked, what would make an HTTP-01
// validation fail for certain:
//   - DNS (acmehint.Diagnose): a host without A/AAAA records, a failed
//     lookup or a CAA record that does not allow the CA gives DNSNotReady. A
//     host resolving to an address that is not on this server only counts
//     when this server has a public address of its own: behind NAT, in a
//     container or behind a CDN that mismatch is expected.
//   - the route (cert.ProbeHTTP01Routes, the HTTPS onboarding probe): the
//     local nginx must route /.well-known/acme-challenge of every host to
//     Nginx UI. A failure, or a challenge port that cannot be bound, gives
//     IssueFailed without contacting the CA.
func probeCertHosts(ctx context.Context, req certRequest) (acmehint.Evidence, error) {
	diagnostics := diagnoseCertHosts(ctx, req.Hosts, acmehint.Options{
		HasIPv6Listen: true,
		CAAIdentities: acmehint.CAAIdentitiesForDirectory(certCADirOf(req.TLS.ACMEUserID)),
	})
	evidence := acmehint.EvidenceFromDiagnostics(diagnostics)

	publicHost := hasPublicAddr(localAddrs())
	var problems []string
	for _, d := range diagnostics {
		switch d.Code {
		case acmehint.CodeDNSNoRecords:
			problems = append(problems, fmt.Sprintf("%s has no A or AAAA record", d.Params["domain"]))
		case acmehint.CodeDNSLookupFailed:
			problems = append(problems, fmt.Sprintf("looking up %s failed: %s", d.Params["domain"], d.Params["error"]))
		case acmehint.CodeCAABlocksCA:
			problems = append(problems, fmt.Sprintf("a CAA record of %s does not allow the certificate authority", d.Params["domain"]))
		case acmehint.CodeDNSPointsElsewhere:
			if publicHost {
				problems = append(problems, fmt.Sprintf("%s resolves to %s, not to this server (%s)",
					d.Params["domain"], d.Params["elsewhere"], d.Params["local"]))
			}
		}
	}
	if len(problems) > 0 {
		return evidence, &certProbeError{Reason: CertReasonDNSNotReady, Err: errors.New(strings.Join(problems, "; "))}
	}

	pctx, cancel := context.WithTimeout(ctx, certRouteProbeTimeout)
	defer cancel()
	results, err := probeHTTP01Routes(pctx, req.Hosts, cert.WithHTTP01ProbeConfigName(req.SiteName()))
	if err != nil {
		return evidence, &certProbeError{Reason: CertReasonIssueFailed, Err: fmt.Errorf("HTTP-01 challenge route check could not run: %w", err)}
	}
	if routeErr := cert.HTTP01RouteCheckError(results, req.SiteName()); routeErr != nil {
		return evidence, &certProbeError{Reason: CertReasonIssueFailed, Err: fmt.Errorf("HTTP-01 challenge route check failed: %w", routeErr)}
	}
	return evidence, nil
}

// certCADir returns the ACME directory of the ACME user, falling back to the
// directory configured in the certificate settings.
func certCADir(acmeUserID uint64) string {
	user, err := (&cert.ConfigPayload{ACMEUserID: acmeUserID}).GetACMEUser()
	if err == nil && user != nil && user.CADir != "" {
		return user.CADir
	}
	return settings.CertSettings.GetCADir()
}

// hasPublicAddr reports whether one of addrs is a public unicast address.
func hasPublicAddr(addrs []netip.Addr) bool {
	sharedAddressSpace := netip.MustParsePrefix("100.64.0.0/10")
	for _, addr := range addrs {
		addr = addr.Unmap()
		if addr.IsGlobalUnicast() && !addr.IsPrivate() && !sharedAddressSpace.Contains(addr) {
			return true
		}
	}
	return false
}

// interfaceAddrs returns the addresses of the network interfaces.
func interfaceAddrs() []netip.Addr {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		if prefix, err := netip.ParsePrefix(addr.String()); err == nil {
			out = append(out, prefix.Addr())
		}
	}
	return out
}
