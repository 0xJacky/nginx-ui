package performance

import "testing"

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
