package services

/*
import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"time"
)

var _ = Describe("OpensearchCLuster data service tests", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 120
		interval = time.Second * 1
	)

	var (
		ClusterClient *OsClusterClient = nil
	)

	/// ------- Creation Check phase -------

	BeforeEach(func() {
		By("Creating open search client ")
		Eventually(func() bool {
			clusterClient, err := NewOsClusterClient(TestClusterUrl, TestClusterUserName, TestClusterPassword)
			if err != nil {
				return false
			}
			ClusterClient = clusterClient
			return true
		}, timeout, interval).Should(BeTrue())
	})
	Context("Data Service Tests logic", func() {
		It("Test Has No Indices With No Replica", func() {
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1,
													"number_of_replicas": 1
													}
												  }
											 }`)
			indexName := "indices-no-rep-test"
			_, err := DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
			success, err := CreateIndex(ClusterClient, indexName, mapping)
			Expect(err).Should(BeNil())
			Expect(success == 200 || success == 201).Should(BeTrue())
			hasNoReplicas, err := HasIndicesWithNoReplica(ClusterClient)
			Expect(err).Should(BeNil())
			Expect(hasNoReplicas).ShouldNot(BeTrue())
			_, err = DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
		})
		It("Test Has Indices With No Replica", func() {
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1,
													"number_of_replicas": 0
													}
												  }
											 }`)
			indexName := "indices-with-rep-test"
			_, err := DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
			hasNoReplicas := false
			success, err := CreateIndex(ClusterClient, indexName, mapping)
			Expect(err).Should(BeNil())
			Expect(success == 200 || success == 201).Should(BeTrue())
			hasNoReplicas, err = HasIndicesWithNoReplica(ClusterClient)
			Expect(err).Should(BeNil())
			Expect(hasNoReplicas).Should(BeTrue())
			_, err = DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
		})
		It("Test Node Exclude", func() {
			nodeExcluded, err := AppendExcludeNodeHost(ClusterClient, "not-exists-node")
			Expect(err).Should(BeNil())
			Expect(nodeExcluded).Should(BeTrue())
			nodeExcluded, err = RemoveExcludeNodeHost(ClusterClient, "not-exists-node")
			Expect(err).Should(BeNil())
			Expect(nodeExcluded).Should(BeTrue())
		})
	})
})*/

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

// TestExtractNodeName verifies that the source node name is correctly extracted from
// the _cat/shards API node field, including during shard relocation when the format
// is "sourceNode -> ip id targetNode".
func TestExtractNodeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "plain node name",
			input:    "opensearch-data-1",
			expected: "opensearch-data-1",
		},
		{
			name:     "shard relocation format - extracts source node",
			input:    "opensearch-data-1 -> 172.31.233.51 4kGSHQhmRQ-83pvvBbTYow opensearch-data-8",
			expected: "opensearch-data-1",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  opensearch-data-2  ",
			expected: "opensearch-data-2",
		},
		{
			name:     "relocation format with leading space",
			input:    "  opensearch-data-0 -> 10.0.0.1 abc123 opensearch-data-5",
			expected: "opensearch-data-0",
		},
		{
			name:     "single token",
			input:    "node-a",
			expected: "node-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNodeName(tt.input)
			if got != tt.expected {
				t.Errorf("extractNodeName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestHasShardsOnNodeFromResponse verifies that shard-to-node matching correctly uses
// the source node name from the _cat/shards API, including during shard relocation when
// the node field format is "sourceNode -> ip id targetNode" (fix for issue #1133).
func TestHasShardsOnNodeFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		shards   []responses.CatShardsResponse
		nodeName string
		want     bool
	}{
		{
			name:     "no shards - returns false",
			shards:   nil,
			nodeName: "opensearch-data-1",
			want:     false,
		},
		{
			name:     "empty shards - returns false",
			shards:   []responses.CatShardsResponse{},
			nodeName: "opensearch-data-1",
			want:     false,
		},
		{
			name: "plain node name match",
			shards: []responses.CatShardsResponse{
				{Index: "idx", Shard: "0", PrimaryOrReplica: "p", State: "STARTED", NodeName: "opensearch-data-1"},
			},
			nodeName: "opensearch-data-1",
			want:     true,
		},
		{
			name: "shard relocation format - match source node",
			shards: []responses.CatShardsResponse{
				{Index: "idx", Shard: "0", PrimaryOrReplica: "p", State: "STARTED", NodeName: "opensearch-data-1 -> 172.31.233.51 4kGSHQhmRQ-83pvvBbTYow opensearch-data-8"},
			},
			nodeName: "opensearch-data-1",
			want:     true,
		},
		{
			name: "shard relocation format - target node should not match when querying by name",
			shards: []responses.CatShardsResponse{
				{Index: "idx", Shard: "0", PrimaryOrReplica: "p", State: "STARTED", NodeName: "opensearch-data-1 -> 172.31.233.51 4kGSHQhmRQ opensearch-data-8"},
			},
			nodeName: "opensearch-data-8",
			want:     false,
		},
		{
			name: "multiple shards - one matches source node",
			shards: []responses.CatShardsResponse{
				{Index: "a", Shard: "0", PrimaryOrReplica: "p", State: "STARTED", NodeName: "other-node"},
				{Index: "b", Shard: "0", PrimaryOrReplica: "p", State: "STARTED", NodeName: "opensearch-data-2 -> 10.0.0.1 xyz opensearch-data-9"},
			},
			nodeName: "opensearch-data-2",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasShardsOnNodeFromResponse(tt.shards, tt.nodeName)
			if got != tt.want {
				t.Errorf("hasShardsOnNodeFromResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestShardsOnNodeFromResponseRelocatingSource is the CheckPodSafeToDelete
// regression for issue #1447: a shard relocating off the node must still count
// as present so the pod is not deleted while it is the relocation source.
func TestShardsOnNodeFromResponseRelocatingSource(t *testing.T) {
	shards := []responses.CatShardsResponse{
		{Index: "idx", Shard: "0", PrimaryOrReplica: "p", State: "RELOCATING", NodeName: "opensearch-data-1 -> 172.31.233.51 4kGSHQhmRQ-83pvvBbTYow opensearch-data-8"},
	}

	onSource := shardsOnNodeFromResponse(shards, "opensearch-data-1")
	if len(onSource) != 1 {
		t.Fatalf("relocating shard should count on source node, got %d", len(onSource))
	}

	onTarget := shardsOnNodeFromResponse(shards, "opensearch-data-8")
	if len(onTarget) != 0 {
		t.Fatalf("relocating shard should not count on target node, got %d", len(onTarget))
	}

	// Raw string equality is what CheckPodSafeToDelete used to do and would
	// miss this shard entirely, reporting the node as empty.
	if shards[0].NodeName == "opensearch-data-1" {
		t.Fatal("test fixture is wrong: raw NodeName should be the relocation string")
	}
}

func TestDetermineUnsupportedClusterSettings(t *testing.T) {
	tests := []struct {
		name                string
		newVersion          string
		wantTransientCount  int
		wantPersistentCount int
	}{
		{
			name:                "2.x upgrade has no settings to delete",
			newVersion:          "2.19.5",
			wantTransientCount:  0,
			wantPersistentCount: 0,
		},
		{
			name:                "3.x upgrade includes archived ISM settings",
			newVersion:          "3.0.0",
			wantTransientCount:  6,
			wantPersistentCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetermineUnsupportedClusterSettings(tt.newVersion)
			if err != nil {
				t.Fatalf("DetermineUnsupportedClusterSettings(%q) returned error: %v", tt.newVersion, err)
			}

			if got.Transient == nil {
				t.Fatalf("Transient settings map is nil")
			}
			if got.Persistent == nil {
				t.Fatalf("Persistent settings map is nil")
			}

			if len(got.Transient) != tt.wantTransientCount {
				t.Fatalf("len(Transient) = %d, want %d", len(got.Transient), tt.wantTransientCount)
			}
			if len(got.Persistent) != tt.wantPersistentCount {
				t.Fatalf("len(Persistent) = %d, want %d", len(got.Persistent), tt.wantPersistentCount)
			}
		})
	}
}

// newTestClient creates an OsClusterClient backed by the given httptest.Server.
// The server must handle GET / (ping) and return a valid main page response.
func newTestClient(t *testing.T, server *httptest.Server) *OsClusterClient {
	t.Helper()
	client, err := NewOsClusterClient(server.URL, "", "", WithTransport(server.Client().Transport))
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}
	return client
}

// jsonResponse writes a JSON body with the given status code.
func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// TestCheckClusterStatusForRestart_YellowAllocationPrimaries verifies that when
// the cluster is yellow and allocation is set to "primaries", the function
// re-enables allocation to "all" BEFORE attempting any per-index shard analysis.
// This is the fix for the deadlock where CatNamedIndicesShards on a system index
// returned 403, blocking the allocation reset code.
func TestCheckClusterStatusForRestart_YellowAllocationPrimaries(t *testing.T) {
	allocationResetCalled := false
	catShardsCalled := false

	mux := http.NewServeMux()

	// GET / — ping / main page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	// GET /_cluster/health — yellow with system index
	mux.HandleFunc("/_cluster/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.ClusterHealthResponse{
			Status:           "yellow",
			UnassignedShards: 5,
			Indices: map[string]responses.IndexHealth{
				".opendistro_security": {Status: "yellow", UnassignedShards: 1},
				"my-index":             {Status: "yellow", UnassignedShards: 4},
			},
		})
	})

	// GET /_cluster/settings — allocation is "primaries"
	mux.HandleFunc("/_cluster/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			jsonResponse(w, 200, responses.FlatClusterSettingsResponse{
				Transient: responses.Settings{
					ClusterRoutingAllocationEnable: "primaries",
				},
			})
		} else if r.Method == http.MethodPut {
			allocationResetCalled = true
			jsonResponse(w, 200, map[string]interface{}{})
		}
	})

	// GET /_cat/shards — should NOT be called in this scenario
	mux.HandleFunc("/_cat/shards/", func(w http.ResponseWriter, r *http.Request) {
		catShardsCalled = true
		// Return 403 to simulate system index permission error
		http.Error(w, `[403 Forbidden] {"error":"no permissions for [] and User [name=admin]"}`, 403)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	ready, msg, err := CheckClusterStatusForRestart(client, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Fatal("expected ready=false, got true")
	}
	if !strings.Contains(msg, "re-enabled shard allocation") {
		t.Fatalf("expected message about re-enabling allocation, got: %q", msg)
	}
	if !allocationResetCalled {
		t.Fatal("expected allocation to be reset to 'all', but PUT was not called")
	}
	if catShardsCalled {
		t.Fatal("CatShards should NOT be called when allocation is still restricted")
	}
}

// TestCheckClusterStatusForRestart_YellowAllocationAll verifies that when
// allocation is already "all" and the cluster is yellow, the function proceeds
// to CheckClusterRestartOnYellow (per-index analysis).
func TestCheckClusterStatusForRestart_YellowAllocationAll(t *testing.T) {
	catShardsCalled := false

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	mux.HandleFunc("/_cluster/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.ClusterHealthResponse{
			Status:           "yellow",
			UnassignedShards: 2,
			Indices: map[string]responses.IndexHealth{
				"my-index": {Status: "yellow", UnassignedShards: 2},
			},
		})
	})

	mux.HandleFunc("/_cluster/settings", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.FlatClusterSettingsResponse{
			Transient: responses.Settings{
				ClusterRoutingAllocationEnable: "all",
			},
		})
	})

	// CatNamedIndicesShards — returns empty shards (no version mismatch)
	mux.HandleFunc("/_cat/shards/", func(w http.ResponseWriter, r *http.Request) {
		catShardsCalled = true
		jsonResponse(w, 200, []responses.CatShardsResponse{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	ready, msg, err := CheckClusterStatusForRestart(client, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Not safe to restart (unassigned shards != stuck count), should wait
	if ready {
		t.Fatal("expected ready=false when cluster yellow with non-stuck replicas")
	}
	if !strings.Contains(msg, "waiting for health to be green") {
		t.Fatalf("unexpected message: %q", msg)
	}
	if !catShardsCalled {
		t.Fatal("expected CatShards to be called when allocation is already 'all'")
	}
}

// TestCheckClusterStatusForRestart_Green verifies the green path returns ready.
func TestCheckClusterStatusForRestart_Green(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	mux.HandleFunc("/_cluster/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.ClusterHealthResponse{Status: "green"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	ready, _, err := CheckClusterStatusForRestart(client, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Fatal("expected ready=true for green cluster")
	}
}

// TestCheckClusterRestartOnYellow_403Skipped verifies that when
// CatNamedIndicesShards returns 403 for a system index, that index is
// skipped and the function does not return an error.
func TestCheckClusterRestartOnYellow_403Skipped(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	mux.HandleFunc("/_cat/shards/", func(w http.ResponseWriter, r *http.Request) {
		// Check which index is being queried
		indexParam := r.URL.Path
		if strings.Contains(indexParam, ".opendistro_security") {
			http.Error(w, `[403 Forbidden] {"error":"no permissions"}`, 403)
			return
		}
		// Return normal shards for other indices
		jsonResponse(w, 200, []responses.CatShardsResponse{
			{Index: "my-index", Shard: "0", PrimaryOrReplica: "r", State: "UNASSIGNED"},
		})
	})

	// _cluster/allocation/explain — needed by DetectShardStuckVersionMismatch
	mux.HandleFunc("/_cluster/allocation/explain", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.AllocationExplainResponse{
			Index:        "my-index",
			Shard:        0,
			CurrentState: "unassigned",
			CanAllocate:  "no",
			NodeAllocationDecisions: []responses.AllocationExplainNodeDecision{
				{
					Decision: "no",
					Deciders: []responses.AllocationDecision{
						{Decider: "node_version", Decision: "NO", Explanation: "node version mismatch"},
					},
				},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	health := responses.ClusterHealthResponse{
		Status:           "yellow",
		UnassignedShards: 2,
		Indices: map[string]responses.IndexHealth{
			".opendistro_security": {Status: "yellow", UnassignedShards: 1},
			"my-index":             {Status: "yellow", UnassignedShards: 1},
		},
	}

	// Should NOT return error despite 403 on system index
	safeToRestart, err := CheckClusterRestartOnYellow(client, health)
	if err != nil {
		t.Fatalf("expected no error but got: %v", err)
	}
	// Cannot be safe because skipped index has unassigned shards not counted as stuck
	if safeToRestart {
		t.Fatal("expected safeToRestart=false because skipped index shards are not counted")
	}
}

// TestCheckClusterRestartOnYellow_NonPermissionError verifies that non-403
// errors from CatNamedIndicesShards are still propagated.
func TestCheckClusterRestartOnYellow_NonPermissionError(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	mux.HandleFunc("/_cat/shards/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `[500 Internal Server Error]`, 500)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	health := responses.ClusterHealthResponse{
		Status:           "yellow",
		UnassignedShards: 1,
		Indices: map[string]responses.IndexHealth{
			"my-index": {Status: "yellow", UnassignedShards: 1},
		},
	}

	_, err := CheckClusterRestartOnYellow(client, health)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// TestCheckClusterStatusForRestart_YellowDrainNodes verifies that when
// drainNodes=true and cluster is yellow, the function returns without
// attempting to reset allocation (original behavior preserved).
func TestCheckClusterStatusForRestart_YellowDrainNodes(t *testing.T) {
	settingsFetched := false

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	mux.HandleFunc("/_cluster/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.ClusterHealthResponse{
			Status:           "yellow",
			UnassignedShards: 2,
			Indices: map[string]responses.IndexHealth{
				"my-index": {Status: "yellow", UnassignedShards: 2},
			},
		})
	})

	mux.HandleFunc("/_cluster/settings", func(w http.ResponseWriter, r *http.Request) {
		settingsFetched = true
		jsonResponse(w, 200, responses.FlatClusterSettingsResponse{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	ready, msg, err := CheckClusterStatusForRestart(client, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Fatal("expected ready=false")
	}
	if !strings.Contains(msg, "drain nodes is enabled") {
		t.Fatalf("unexpected message: %q", msg)
	}
	if settingsFetched {
		t.Fatal("should not fetch cluster settings when drainNodes=true and cluster is yellow")
	}
}

// TestCheckClusterStatusForRestart_RedAllocationRestricted verifies the red
// cluster path still resets allocation.
func TestCheckClusterStatusForRestart_RedAllocationRestricted(t *testing.T) {
	allocationResetCalled := false

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"name":         "test",
			"cluster_name": "test",
			"version":      map[string]interface{}{"number": "2.11.0", "distribution": "opensearch"},
		})
	})

	mux.HandleFunc("/_cluster/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, responses.ClusterHealthResponse{Status: "red"})
	})

	mux.HandleFunc("/_cluster/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			jsonResponse(w, 200, responses.FlatClusterSettingsResponse{
				Transient: responses.Settings{
					ClusterRoutingAllocationEnable: "primaries",
				},
			})
		} else if r.Method == http.MethodPut {
			allocationResetCalled = true
			jsonResponse(w, 200, map[string]interface{}{})
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)

	ready, msg, err := CheckClusterStatusForRestart(client, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Fatal("expected ready=false for red cluster")
	}
	_ = msg
	if !allocationResetCalled {
		t.Fatal("expected allocation to be reset for red cluster with restricted allocation")
	}
}
