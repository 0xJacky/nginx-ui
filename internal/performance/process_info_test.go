package performance

import "testing"

func TestIsNginxProcessName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "nginx", want: true},
		{name: "NGINX", want: true},
		{name: "nginx.exe", want: true},
		// Official Docker image: Nginx UI runs next to nginx, and its process
		// name contains "nginx" as a substring (#1704).
		{name: "nginx-ui", want: false},
		{name: "s6-supervise", want: false},
		{name: "sh", want: false},
		{name: "openresty", want: false},
		{name: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNginxProcessName(tt.name); got != tt.want {
				t.Errorf("isNginxProcessName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsNginxMasterCmdline(t *testing.T) {
	tests := []struct {
		name    string
		cmdline string
		want    bool
	}{
		{
			name:    "retitled master with absolute path",
			cmdline: "nginx: master process /usr/sbin/nginx -g daemon off; ",
			want:    true,
		},
		{
			name:    "retitled master from PATH",
			cmdline: "nginx: master process nginx -g daemon off; ",
			want:    true,
		},
		{
			name:    "non-retitled master fallback",
			cmdline: "/usr/sbin/nginx",
			want:    true,
		},
		{
			// Nginx UI's own command line still matches the non-retitled
			// fallback; isNginxProcessName is what excludes it in the scan
			// (#1704), as TestHostMasterDetectionOfficialImage verifies.
			name:    "nginx-ui command line matches fallback only",
			cmdline: "/usr/local/bin/nginx-ui --config /etc/nginx-ui/app.ini",
			want:    true,
		},
		{name: "worker", cmdline: "nginx: worker process ", want: false},
		{name: "cache manager", cmdline: "nginx: cache manager process ", want: false},
		{name: "cache loader", cmdline: "nginx: cache loader process ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNginxMasterCmdline(tt.cmdline); got != tt.want {
				t.Errorf("isNginxMasterCmdline(%q) = %v, want %v", tt.cmdline, got, tt.want)
			}
		})
	}
}

// TestHostMasterDetectionOfficialImage guards the regression from #1704: in
// the official Docker image the nginx-ui service starts without dependencies
// while the nginx service waits for init-config, so nginx-ui gets a lower PID
// than the nginx master. The first-match-wins master scan visits nginx-ui
// first and must not select it as the master, otherwise every worker's parent
// PID check fails and the dashboard reports zero workers.
func TestHostMasterDetectionOfficialImage(t *testing.T) {
	processes := []struct {
		pid     int32
		name    string
		cmdline string
	}{
		{1, "s6-svscan", "/init"},
		{7, "sh", "/command/with-contenv sh /etc/s6-overlay/s6-rc.d/nginx-ui/run"},
		{8, "nginx-ui", "/usr/local/bin/nginx-ui --config /etc/nginx-ui/app.ini"},
		{9, "sh", "/command/with-contenv sh /etc/s6-overlay/s6-rc.d/nginx/run"},
		{10, "nginx", "nginx: master process nginx -g daemon off; "},
		{11, "nginx", "nginx: worker process "},
		{12, "nginx", "nginx: worker process "},
		{13, "nginx", "nginx: worker process "},
	}

	var masterPID int32 = -1
	for _, p := range processes {
		if isNginxProcessName(p.name) && isNginxMasterCmdline(p.cmdline) && p.pid > 0 {
			masterPID = p.pid
			break
		}
	}

	const wantMasterPID = 10
	if masterPID != wantMasterPID {
		t.Fatalf("masterPID = %d, want %d (the real nginx master, not nginx-ui)", masterPID, wantMasterPID)
	}

	workers := 0
	for _, p := range processes {
		if p.pid == masterPID || !isNginxProcessName(p.name) {
			continue
		}
		if p.cmdline == "nginx: worker process " {
			workers++
		}
	}
	if workers != 3 {
		t.Errorf("workers = %d, want 3", workers)
	}
}

func TestParseContainerNginxProcesses(t *testing.T) {
	tests := []struct {
		name                              string
		procList                          string
		wantMaster, wantWorker, wantCache int
	}{
		{
			name: "master with workers and cache",
			procList: "nginx: master process /usr/sbin/nginx -g daemon off; \n" +
				"nginx: worker process \n" +
				"nginx: worker process \n" +
				"nginx: worker process \n" +
				"nginx: worker process \n" +
				"nginx: cache manager process \n" +
				"sh -c for p in /proc/cmdline\n" +
				"\n",
			wantMaster: 1,
			wantWorker: 4,
			wantCache:  1,
		},
		{
			name: "cache loader counted as cache",
			procList: "nginx: master process nginx -g daemon off; \n" +
				"nginx: cache loader process \n",
			wantMaster: 1,
			wantWorker: 0,
			wantCache:  1,
		},
		{
			name:       "no nginx processes",
			procList:   "bash \nsleep 100 \n",
			wantMaster: 0,
			wantWorker: 0,
			wantCache:  0,
		},
		{
			name:       "empty output",
			procList:   "",
			wantMaster: 0,
			wantWorker: 0,
			wantCache:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseContainerNginxProcesses(tt.procList)
			if info.Master != tt.wantMaster {
				t.Errorf("Master = %d, want %d", info.Master, tt.wantMaster)
			}
			if info.Workers != tt.wantWorker {
				t.Errorf("Workers = %d, want %d", info.Workers, tt.wantWorker)
			}
			if info.Cache != tt.wantCache {
				t.Errorf("Cache = %d, want %d", info.Cache, tt.wantCache)
			}
		})
	}
}
