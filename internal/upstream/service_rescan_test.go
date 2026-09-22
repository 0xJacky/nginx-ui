package upstream

import "testing"

const rescanTestConfig = `
upstream api {
    server 127.0.0.1:9000;
    server 127.0.0.1:9001;
}
server {
    listen 80;
    location / {
        proxy_pass http://api;
    }
}`

func setAvailabilityForTest(service *Service, results map[string]*Status) {
	service.targetsMutex.Lock()
	defer service.targetsMutex.Unlock()
	service.availabilityMap = results
}

// The config scanner rescans every config periodically and on file events. A
// rescan of an unchanged config must not throw away the availability results
// of its targets, or every client is told they have no status until the next
// availability test runs.
func TestScanForProxyTargets_KeepsAvailabilityWhenConfigIsRescanned(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	t.Cleanup(func() {
		service.ClearTargets()
	})

	if err := scanForProxyTargets("site.conf", []byte(rescanTestConfig)); err != nil {
		t.Fatalf("initial scan failed: %v", err)
	}

	setAvailabilityForTest(service, map[string]*Status{
		"127.0.0.1:9000": {Online: true, Latency: 1},
		"127.0.0.1:9001": {Online: false},
	})

	if err := scanForProxyTargets("site.conf", []byte(rescanTestConfig)); err != nil {
		t.Fatalf("rescan failed: %v", err)
	}

	results := service.GetAvailabilityMap()
	for _, key := range []string{"127.0.0.1:9000", "127.0.0.1:9001"} {
		if _, ok := results[key]; !ok {
			t.Errorf("rescan of an unchanged config dropped the availability result for %s; got %v", key, results)
		}
	}
	if len(service.GetTargets()) != 2 {
		t.Errorf("expected both targets to stay registered, got %+v", service.GetTargets())
	}
}

// Keeping results across a rescan must not keep them for a target the config
// no longer references.
func TestScanForProxyTargets_DropsAvailabilityOfRemovedTarget(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	t.Cleanup(func() {
		service.ClearTargets()
	})

	if err := scanForProxyTargets("site.conf", []byte(rescanTestConfig)); err != nil {
		t.Fatalf("initial scan failed: %v", err)
	}

	setAvailabilityForTest(service, map[string]*Status{
		"127.0.0.1:9000": {Online: true, Latency: 1},
		"127.0.0.1:9001": {Online: false},
	})

	updatedConfig := `
upstream api {
    server 127.0.0.1:9000;
}
server {
    listen 80;
    location / {
        proxy_pass http://api;
    }
}`

	if err := scanForProxyTargets("site.conf", []byte(updatedConfig)); err != nil {
		t.Fatalf("rescan failed: %v", err)
	}

	results := service.GetAvailabilityMap()
	if _, ok := results["127.0.0.1:9000"]; !ok {
		t.Errorf("expected the still-referenced target to keep its result; got %v", results)
	}
	if _, ok := results["127.0.0.1:9001"]; ok {
		t.Errorf("expected the removed target to lose its result; got %v", results)
	}
}
