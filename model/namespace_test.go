package model

import "testing"

func TestNamespaceIsRemoteDeploy(t *testing.T) {
	var missing *Namespace
	if missing.IsRemoteDeploy() {
		t.Fatal("a site without a namespace deploys locally")
	}

	if (&Namespace{DeployMode: DeployModeLocal}).IsRemoteDeploy() {
		t.Fatal("local deploy mode must not be treated as remote")
	}

	if !(&Namespace{DeployMode: DeployModeRemote}).IsRemoteDeploy() {
		t.Fatal("remote deploy mode must be detected")
	}
}

func TestNamespaceIsAutoSync(t *testing.T) {
	var missing *Namespace
	if missing.IsAutoSync() {
		t.Fatal("a missing namespace never syncs automatically")
	}

	if (&Namespace{SyncStrategy: SyncStrategyManual}).IsAutoSync() {
		t.Fatal("manual strategy must not sync automatically")
	}

	if !(&Namespace{SyncStrategy: SyncStrategyAuto}).IsAutoSync() {
		t.Fatal("auto strategy must sync automatically")
	}
}

func TestNamespaceEffectiveSyncInterval(t *testing.T) {
	var missing *Namespace
	if got := missing.EffectiveSyncInterval(); got != DefaultSyncIntervalMinutes {
		t.Fatalf("got %d, want %d", got, DefaultSyncIntervalMinutes)
	}

	if got := (&Namespace{SyncIntervalMinutes: 0}).EffectiveSyncInterval(); got != DefaultSyncIntervalMinutes {
		t.Fatalf("got %d, want %d", got, DefaultSyncIntervalMinutes)
	}

	if got := (&Namespace{SyncIntervalMinutes: -5}).EffectiveSyncInterval(); got != DefaultSyncIntervalMinutes {
		t.Fatalf("got %d, want %d", got, DefaultSyncIntervalMinutes)
	}

	if got := (&Namespace{SyncIntervalMinutes: 5}).EffectiveSyncInterval(); got != 5 {
		t.Fatalf("got %d, want 5", got)
	}
}
