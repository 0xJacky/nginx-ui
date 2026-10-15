package ingress

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	certDir := t.TempDir()
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
