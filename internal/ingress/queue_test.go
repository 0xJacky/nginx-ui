package ingress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func upsertBasic(t *testing.T, name, host string) *model.Ingress {
	t.Helper()
	ing, err := Upsert(context.Background(), name, UpsertRequest{Owner: "tester", Spec: basicSpec(host)})
	require.NoError(t, err)
	return ing
}

func waitApplied(t *testing.T, ing *model.Ingress) *model.Ingress {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := WaitForGeneration(ctx, ing.Name, ing.Generation)
	require.NoError(t, err)
	return got
}

func TestWorkerCoalescesEnqueuesIntoOneApply(t *testing.T) {
	env := setupEnv(t)
	coalesceQuiet = 100 * time.Millisecond
	var stored []*model.Ingress
	for i := range 5 {
		stored = append(stored, upsertBasic(t, fmt.Sprintf("c%d", i), fmt.Sprintf("c%d.example.com", i)))
	}
	env.startWorker(t)

	for _, ing := range stored {
		got := waitApplied(t, ing)
		assert.Equal(t, model.IngressPhaseReady, got.Status.Phase, got.Name)
		applied, ok := FindCondition(&got.Status, model.IngressConditionConfigApplied)
		require.True(t, ok)
		assert.True(t, applied.Status)
		assert.Equal(t, applyReasonApplied, applied.Reason)
		cert, _ := FindCondition(&got.Status, model.IngressConditionCertReady)
		assert.True(t, cert.Status)
		assert.Equal(t, applyReasonCertNotRequired, cert.Reason)
		health, _ := FindCondition(&got.Status, model.IngressConditionBackendsHealthy)
		assert.Equal(t, ReasonUnknown, health.Reason)
		assert.NotEmpty(t, got.Status.ConfigHash)
		assert.NotNil(t, got.Status.LastAppliedAt)
		assert.Len(t, got.Status.Servers, 1)
	}
	assert.Equal(t, 1, env.tests())
	assert.Equal(t, 1, env.reloads())
}

func TestWorkerIsolatesTheIngressThatBreaksTheTest(t *testing.T) {
	env := setupEnv(t)
	good := upsertBasic(t, "good", "good.example.com")
	other := upsertBasic(t, "other", "other.example.com")
	spec := basicSpec("bad.example.com")
	spec.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-Marker", Value: brokenMarker}}
	bad, err := Upsert(context.Background(), "bad", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)
	env.startWorker(t)

	for _, ing := range []*model.Ingress{good, other} {
		got := waitApplied(t, ing)
		assert.Equal(t, model.IngressPhaseReady, got.Status.Phase, got.Name)
		assert.True(t, env.exists("sites-enabled", SiteName(got.Name)))
	}
	got := waitApplied(t, bad)
	assert.Equal(t, model.IngressPhaseError, got.Status.Phase)
	applied, _ := FindCondition(&got.Status, model.IngressConditionConfigApplied)
	assert.False(t, applied.Status)
	assert.Equal(t, applyReasonTestFailed, applied.Reason)
	assert.Contains(t, applied.Message, "broken config")
	assert.Equal(t, bad.Generation, got.Status.ObservedGeneration)
	assert.False(t, env.exists("sites-available", "ingress-bad"))
}

// Issuance runs in memory only, so a restarted worker resumes it for the
// ACME ingresses that were applied before the restart.
func TestWorkerStartEnsuresAppliedACMECertificates(t *testing.T) {
	env := setupEnv(t)
	spec := basicSpec("resume.example.com")
	spec.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME}
	applied := &model.Ingress{Name: "resume", Owner: "tester", Spec: spec, Generation: 2,
		Status: model.IngressStatus{ObservedGeneration: 2, Phase: model.IngressPhaseDegraded}}
	require.NoError(t, model.UseDB().Create(applied).Error)
	plain := &model.Ingress{Name: "plain", Owner: "tester", Spec: basicSpec("plain.example.com"), Generation: 1,
		Status: model.IngressStatus{ObservedGeneration: 1, Phase: model.IngressPhaseReady}}
	require.NoError(t, model.UseDB().Create(plain).Error)

	env.startWorker(t)

	eventually(t, func() bool {
		ensured, _ := env.certs.snapshot()
		return len(ensured) == 1 && ensured[0] == "resume"
	}, "Ensure was not called for the applied ACME ingress")
}

func TestWorkerEnsuresACMECertificates(t *testing.T) {
	env := setupEnv(t)
	spec := basicSpec("acme.example.com")
	spec.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME, RedirectHTTPS: true}
	env.certs.set("acme", CertState{Reason: "DNSNotReady", Message: "acme.example.com does not resolve here"})
	ing, err := Upsert(context.Background(), "acme", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)
	env.startWorker(t)

	got := waitApplied(t, ing)
	assert.Equal(t, model.IngressPhaseDegraded, got.Status.Phase)
	cert, _ := FindCondition(&got.Status, model.IngressConditionCertReady)
	assert.False(t, cert.Status)
	assert.Equal(t, "DNSNotReady", cert.Reason)
	assert.NotContains(t, env.read(t, "sites-available", "ingress-acme"), "443")
	eventually(t, func() bool {
		ensured, _ := env.certs.snapshot()
		return len(ensured) == 1 && ensured[0] == "acme"
	}, "Ensure was not called")

	// The certificate manager reports the issued certificate and enqueues.
	notAfter := time.Now().Add(90 * 24 * time.Hour).UTC().Truncate(time.Second)
	state := readyCert()
	state.NotAfter = &notAfter
	env.certs.set("acme", state)
	Enqueue("acme")
	eventually(t, func() bool {
		cur, err := Get("acme")
		return err == nil && cur.Status.CertID == state.CertID
	}, "certificate was not recorded")
	cur, err := Get("acme")
	require.NoError(t, err)
	assert.Equal(t, model.IngressPhaseReady, cur.Status.Phase)
	require.NotNil(t, cur.Status.CertNotAfter)
	assert.True(t, notAfter.Equal(*cur.Status.CertNotAfter))
	assert.Contains(t, env.read(t, "sites-available", "ingress-acme"), "listen 443 ssl;")
}

func TestWaitForGenerationTimesOut(t *testing.T) {
	setupEnv(t)
	ing := upsertBasic(t, "slow", "slow.example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := WaitForGeneration(ctx, ing.Name, ing.Generation)
	requireCode(t, err, ErrWaitTimeout)

	_, err = WaitForGeneration(context.Background(), "missing", 1)
	requireCode(t, err, ErrNotFound)
}

func TestWaitForGenerationWakesOnStatusUpdate(t *testing.T) {
	setupEnv(t)
	ing := upsertBasic(t, "wake", "wake.example.com")
	go func() {
		time.Sleep(30 * time.Millisecond)
		_, _ = UpdateStatus("wake", func(cur *model.Ingress) { cur.Status.ObservedGeneration = cur.Generation })
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := WaitForGeneration(ctx, ing.Name, ing.Generation)
	require.NoError(t, err)
	assert.Equal(t, ing.Generation, got.Status.ObservedGeneration)
}

func TestDeleteRemovesFilesRowAndForgetsCertificate(t *testing.T) {
	env := setupEnv(t)
	env.startWorker(t)
	ing := waitApplied(t, upsertBasic(t, "del", "del.example.com"))
	require.True(t, env.exists("sites-enabled", "ingress-del"))

	requireCode(t, Delete(context.Background(), "del", "someone-else", false, false), ErrOwnerMismatch)
	require.NoError(t, Delete(context.Background(), ing.Name, "tester", false, false))

	assert.False(t, env.exists("sites-available", "ingress-del"))
	assert.False(t, env.exists("sites-enabled", "ingress-del"))
	assert.False(t, env.exists("conf.d", "upstream-ig-del-app.conf"))
	_, err := Get("del")
	requireCode(t, err, ErrNotFound)
	_, forgotten := env.certs.snapshot()
	assert.Equal(t, []string{"del"}, forgotten)
	requireCode(t, Delete(context.Background(), "del", "tester", false, false), ErrNotFound)
}

func TestDeletePurgesUnusedACMECertificate(t *testing.T) {
	env := setupEnv(t)
	certDir := t.TempDir() + "/purge.example.com_RSA2048"
	require.NoError(t, os.Mkdir(certDir, 0o755))
	certPath, keyPath := certDir+"/fullchain.cer", certDir+"/private.key"
	require.NoError(t, os.WriteFile(certPath, []byte("cert"), 0o644))
	require.NoError(t, os.WriteFile(keyPath, []byte("key"), 0o600))
	cert := &model.Cert{Name: "purge", SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
	require.NoError(t, model.UseDB().Create(cert).Error)

	spec := basicSpec("purge.example.com")
	spec.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME}
	env.certs.set("purge", CertState{Ready: true, CertID: cert.ID, CertPath: certPath, KeyPath: keyPath})
	ing, err := Upsert(context.Background(), "purge", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)
	env.startWorker(t)
	waitApplied(t, ing)

	require.NoError(t, Delete(context.Background(), "purge", "tester", false, true))
	var count int64
	model.UseDB().Model(&model.Cert{}).Where("id = ?", cert.ID).Count(&count)
	assert.Zero(t, count)
	_, err = os.Stat(certPath)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(keyPath)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(certDir)
	assert.True(t, os.IsNotExist(err), "the empty certificate directory is removed")
}

func TestWorkerAppliesUnappliedIngressesOnStart(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("boot", "boot.example.com")
	ing.Status.Phase = model.IngressPhaseApplying
	require.NoError(t, model.UseDB().Create(ing).Error)
	queue.take()

	env.startWorker(t)
	got := waitApplied(t, ing)
	assert.Equal(t, model.IngressPhaseReady, got.Status.Phase)
}

// deleteAsync runs Delete in the background and returns its outcome channel.
func deleteAsync(name string) <-chan error {
	out := make(chan error, 1)
	go func() {
		out <- Delete(context.Background(), name, "tester", false, false)
	}()
	return out
}

func waitQueuedDelete(t *testing.T, name string) {
	t.Helper()
	eventually(t, func() bool {
		queue.mu.Lock()
		defer queue.mu.Unlock()
		_, ok := queue.deletes[name]
		return ok
	}, "delete was not queued")
}

func receive(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("no outcome")
		return nil
	}
}

func TestUpsertSupersedesQueuedDelete(t *testing.T) {
	env := setupEnv(t)
	upsertBasic(t, "keep", "keep.example.com")
	done := deleteAsync("keep")
	waitQueuedDelete(t, "keep")

	// The write lands before the worker picked the delete up: it wins.
	ing := upsertBasic(t, "keep", "keep.example.com")
	requireCode(t, receive(t, done), ErrDeleteConflict)

	env.startWorker(t)
	got := waitApplied(t, ing)
	assert.Equal(t, model.IngressPhaseReady, got.Status.Phase)
	assert.True(t, env.exists("sites-enabled", "ingress-keep"))
}

func TestDeleteKeepsIngressUpdatedWhileDeleting(t *testing.T) {
	cases := map[string]func(t *testing.T){
		// Upsert flags the delete the worker is processing.
		"superseded": func(t *testing.T) {
			_, err := Upsert(context.Background(), "race", UpsertRequest{Owner: "tester", Spec: basicSpec("race2.example.com")})
			require.NoError(t, err)
		},
		// The generation moved without passing through the store.
		"generation moved": func(t *testing.T) {
			require.NoError(t, model.UseDB().Model(&model.Ingress{}).Where("name = ?", "race").
				Update("generation", 5).Error)
		},
	}
	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			env := setupEnv(t)
			stop := env.runWorker()
			waitApplied(t, upsertBasic(t, "race", "race.example.com"))
			stop()

			// Hand the delete to a batch by hand so the write can land while
			// the batch runs.
			storeMu.Lock()
			done, err := requestDelete("race", "tester", false, false)
			storeMu.Unlock()
			require.NoError(t, err)
			applies, deletes := queue.take()
			write(t)
			processBatch(context.Background(), applies, deletes)
			queue.done()

			requireCode(t, receive(t, done), ErrDeleteConflict)
			cur, err := Get("race")
			require.NoError(t, err, "the row must survive")
			_, forgotten := env.certs.snapshot()
			assert.Empty(t, forgotten)

			// The apply that restores the files was queued.
			env.startWorker(t)
			eventually(t, func() bool { return env.exists("sites-enabled", "ingress-race") },
				"files were not restored")
			eventually(t, func() bool {
				got, err := Get("race")
				return err == nil && got.Status.ObservedGeneration == cur.Generation &&
					got.Status.Phase == model.IngressPhaseReady
			}, "the ingress was not applied again")
		})
	}
}

func TestSyncOwnerPruneLosesToALaterUpsert(t *testing.T) {
	env := setupEnv(t)
	upsertBasic(t, "stay", "stay.example.com")
	upsertBasic(t, "pruned", "pruned.example.com")

	_, err := SyncOwner(context.Background(), "tester", SyncRequest{Items: []SyncItem{
		{Name: "stay", Spec: basicSpec("stay.example.com")},
	}}, true)
	require.NoError(t, err)
	// The prune was queued with the generation it saw, before storeMu was
	// released.
	queue.mu.Lock()
	job := queue.deletes["pruned"]
	queue.mu.Unlock()
	require.NotNil(t, job)
	assert.Equal(t, int64(1), job.generation)

	ing := upsertBasic(t, "pruned", "pruned.example.com")
	queue.mu.Lock()
	_, stillQueued := queue.deletes["pruned"]
	queue.mu.Unlock()
	assert.False(t, stillQueued)

	env.startWorker(t)
	waitApplied(t, ing)
	assert.True(t, env.exists("sites-enabled", "ingress-pruned"))
}

func TestSyncOwnerStoresAllItemsOrNone(t *testing.T) {
	setupEnv(t)
	db := model.UseDB()
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_boom", func(tx *gorm.DB) {
		if ing, ok := tx.Statement.Dest.(*model.Ingress); ok && ing.Name == "boom" {
			_ = tx.AddError(errors.New("disk full"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test:fail_boom") })

	_, err := SyncOwner(context.Background(), "tester", SyncRequest{Items: []SyncItem{
		{Name: "first", Spec: basicSpec("first.example.com")},
		{Name: "boom", Spec: basicSpec("boom.example.com")},
	}}, false)
	require.Error(t, err)

	items, err := List("tester")
	require.NoError(t, err)
	assert.Empty(t, items, "a failed sync stores nothing")
	queue.mu.Lock()
	assert.Empty(t, queue.applies)
	queue.mu.Unlock()
}

// A fresh apply refreshes the backend health right away instead of leaving
// it to the next reconcile pass, and the new servers start unchecked.
func TestWorkerRefreshesHealthAfterApply(t *testing.T) {
	env := setupEnv(t)
	good := upsertBasic(t, "fresh", "fresh.example.com")
	spec := basicSpec("broken.example.com")
	spec.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-Marker", Value: brokenMarker}}
	bad, err := Upsert(context.Background(), "broken", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)
	env.startWorker(t)

	got := waitApplied(t, good)
	waitApplied(t, bad)
	require.Len(t, got.Status.Servers, 1)
	assert.False(t, got.Status.Servers[0].Checked)
	eventually(t, func() bool {
		names := env.refreshedNames()
		return len(names) == 1 && names[0] == "fresh"
	}, "health was not refreshed for the applied ingress only")
}

func TestRecordBackendHealthMarksCheckedServers(t *testing.T) {
	setupEnv(t)
	upsertBasic(t, "probe", "probe.example.com")
	original := reconcileAvailability
	t.Cleanup(func() { reconcileAvailability = original })

	reconcileAvailability = func() (map[string]*upstream.Status, bool) { return nil, true }
	require.NoError(t, recordBackendHealth("probe"))
	got, err := Get("probe")
	require.NoError(t, err)
	require.Len(t, got.Status.Servers, 1)
	assert.False(t, got.Status.Servers[0].Checked)
	health, _ := FindCondition(&got.Status, model.IngressConditionBackendsHealthy)
	assert.Equal(t, ReasonUnknown, health.Reason)

	reconcileAvailability = func() (map[string]*upstream.Status, bool) {
		return map[string]*upstream.Status{"127.0.0.1:24001": {Online: true}}, true
	}
	require.NoError(t, recordBackendHealth("probe"))
	got, err = Get("probe")
	require.NoError(t, err)
	assert.Equal(t, []model.IngressServerStatus{
		{Backend: "app", Address: "127.0.0.1:24001", Healthy: true, Checked: true},
	}, got.Status.Servers)
	health, _ = FindCondition(&got.Status, model.IngressConditionBackendsHealthy)
	assert.Equal(t, reconcileReasonAllHealthy, health.Reason)

	assert.NoError(t, recordBackendHealth("missing"))
}

// A delete without purge_cert keeps the certificate records for a re-create
// but stops renewing them.
func TestDeleteKeepsCertificateWithoutRenewal(t *testing.T) {
	env := setupEnv(t)
	cert := &model.Cert{Name: "kept", Filename: SiteName("kept"), AutoCert: model.AutoCertEnabled}
	require.NoError(t, model.UseDB().Create(cert).Error)
	env.startWorker(t)
	ing := waitApplied(t, upsertBasic(t, "kept", "kept.example.com"))

	require.NoError(t, Delete(context.Background(), ing.Name, "tester", false, false))
	var row model.Cert
	require.NoError(t, model.UseDB().Where("id = ?", cert.ID).First(&row).Error)
	assert.Equal(t, model.AutoCertDisabled, row.AutoCert)
}
