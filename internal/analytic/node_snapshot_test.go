package analytic

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
)

func TestSnapshotNodeMapIsolation(t *testing.T) {
	nodeMapMu.Lock()
	original := NodeMap
	NodeMap = make(TNodeMap)
	NodeMap[1] = &Node{
		Node: &model.Node{
			Model: model.Model{ID: 1},
			Name:  "node-1",
			URL:   "https://example.com",
		},
		NodeStat: NodeStat{
			Status: true,
			UpstreamStatusMap: map[string]*upstream.Status{
				"default": {
					Online:  true,
					Latency: 5,
				},
			},
		},
		NodeInfo: NodeInfo{
			Version: "1.0.0",
		},
	}
	nodeMapMu.Unlock()

	t.Cleanup(func() {
		nodeMapMu.Lock()
		NodeMap = original
		nodeMapMu.Unlock()
	})

	snapshot := SnapshotNodeMap()

	nodeMapMu.Lock()
	NodeMap[1].Status = false
	NodeMap[1].UpstreamStatusMap["default"].Online = false
	NodeMap[1].Node.Name = "mutated"
	nodeMapMu.Unlock()

	cloned := snapshot[1]
	if cloned == nil {
		t.Fatalf("expected snapshot entry for node 1")
	}

	if !cloned.Status {
		t.Fatalf("expected snapshot status to remain true, got false")
	}

	upstreamStatus, ok := cloned.UpstreamStatusMap["default"]
	if !ok || upstreamStatus == nil {
		t.Fatalf("expected upstream status in snapshot")
	}
	if !upstreamStatus.Online {
		t.Fatalf("expected upstream online in snapshot")
	}

	if cloned.Node == nil {
		t.Fatalf("expected cloned node metadata")
	}
	if cloned.Node.Name != "node-1" {
		t.Fatalf("expected cloned node name to remain 'node-1', got %s", cloned.Node.Name)
	}
}

func TestGetNodeReturnsClonedData(t *testing.T) {
	originalDBNode := &model.Node{
		Model: model.Model{ID: 2},
		Name:  "db-node",
		URL:   "https://cluster.local",
		Token: "secret",
	}

	nodeMapMu.Lock()
	original := NodeMap
	NodeMap = make(TNodeMap)
	NodeMap[2] = &Node{
		Node: &model.Node{
			Model: model.Model{ID: 2},
			Name:  "cached-node",
		},
		NodeStat: NodeStat{
			Status: true,
		},
	}
	nodeMapMu.Unlock()

	t.Cleanup(func() {
		nodeMapMu.Lock()
		NodeMap = original
		nodeMapMu.Unlock()
	})

	result := GetNode(originalDBNode)
	if result == nil {
		t.Fatalf("expected GetNode result")
	}
	if result.Node == nil {
		t.Fatalf("expected result node metadata")
	}

	if result.Node.Name != "db-node" {
		t.Fatalf("expected node name from DB copy, got %s", result.Node.Name)
	}

	nodeMapMu.Lock()
	NodeMap[2].Node.Name = "mutated-cache"
	nodeMapMu.Unlock()

	if result.Node.Name != "db-node" {
		t.Fatalf("expected result node name to remain 'db-node', got %s", result.Node.Name)
	}

	result.Node.Name = "updated-result"

	nodeMapMu.RLock()
	if NodeMap[2].Node.Name == "updated-result" {
		t.Fatalf("expected NodeMap to remain isolated from result mutation")
	}
	nodeMapMu.RUnlock()
}
