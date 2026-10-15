package site

import (
	"context"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// Seams for tests. Production code must treat them as read-only.
var (
	issueWithRecord        = cert.IssueWithRecord
	syncIssuedCertificate  = syncCertificateToNodes
	newIssueCertificateLog = cert.NewLogger
)

// certificateIssuances serializes issuances per configuration name. Two runs
// for the same name would share one cert record (and its files), so the
// second one waits for the first instead of interleaving with it. Runs for
// different names are serialized further down by the certificate lock.
var certificateIssuances = struct {
	sync.Mutex
	active map[string]chan struct{}
}{active: map[string]chan struct{}{}}

// lockCertificateIssuance waits until no other issuance holds name, or ctx ends.
func lockCertificateIssuance(ctx context.Context, name string) (release func(), err error) {
	for {
		certificateIssuances.Lock()
		busy, ok := certificateIssuances.active[name]
		if !ok {
			done := make(chan struct{})
			certificateIssuances.active[name] = done
			certificateIssuances.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					certificateIssuances.Lock()
					delete(certificateIssuances.active, name)
					certificateIssuances.Unlock()
					close(done)
				})
			}, nil
		}
		certificateIssuances.Unlock()

		select {
		case <-busy:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// IssueCertificate issues the ACME certificate described by payload for the
// configuration name and keeps its cert record in sync (cert.IssueWithRecord),
// then pushes the certificate to the nodes it is synced to. It is the
// transport-independent entry point shared by the issuance websocket and the
// ingress certificate manager.
//
// The payload key type and identifiers are normalized in place. Set
// payload.ConfigName to run the pre-issuance HTTP-01 route check against that
// site; leave it empty when the caller has probed the route itself. An ACME
// user id of 0 selects the default ACME user. log may be nil, in which case a
// logger that only persists to the cert record is used.
//
// ctx bounds the wait for a concurrent issuance of the same name; once the
// issuance has started it runs to completion. The returned record is nil only
// when the request was rejected before a record was written; otherwise it is
// returned together with the issuance error, if any.
func IssueCertificate(ctx context.Context, name string, payload *cert.ConfigPayload, log *cert.Logger) (*model.Cert, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	payload.KeyType = payload.GetKeyType()
	if err := cert.NormalizeAndValidateIdentifiers(payload); err != nil {
		return nil, err
	}

	release, err := lockCertificateIssuance(ctx, name)
	if err != nil {
		return nil, err
	}
	defer release()

	if log == nil {
		log = newIssueCertificateLog()
		defer log.Close()
	}

	certModel, err := issueWithRecord(name, payload, log)
	if err != nil {
		return certModel, err
	}

	syncIssuedCertificate(certModel.ID)
	return certModel, nil
}

// syncCertificateToNodes pushes a freshly issued certificate to the nodes that
// serve it. A reissue keeps the file paths, so no site configuration changes
// and no site sync would carry the new files to those nodes.
func syncCertificateToNodes(id uint64) {
	issued, err := query.Cert.FirstByID(id)
	if err != nil {
		logger.Error(err)
		return
	}
	if err = cert.SyncToRemoteServer(issued); err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
	}
}
