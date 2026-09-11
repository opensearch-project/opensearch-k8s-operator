package services

import (
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/jarcoal/httpmock"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

func TestSoleActiveCopyOnNode(t *testing.T) {
	sh := func(idx, s, pr, state, node string) responses.CatShardsResponse {
		return responses.CatShardsResponse{Index: idx, Shard: s, PrimaryOrReplica: pr, State: state, NodeName: node}
	}
	tests := []struct {
		name   string
		shards []responses.CatShardsResponse
		node   string
		want   bool
	}{
		{"primary with unassigned replica", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n0"), sh("a", "0", "r", "UNASSIGNED", "")}, "n0", true},
		{"replica started elsewhere", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n0"), sh("a", "0", "r", "STARTED", "n1")}, "n0", false},
		{"zero replicas behaves like green", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n0")}, "n0", false},
		{"replica on candidate, primary elsewhere, extra replica unassigned", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n1"), sh("a", "0", "r", "STARTED", "n0"), sh("a", "0", "r", "UNASSIGNED", "")}, "n0", false},
		{"candidate holds only an initializing copy", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n1"), sh("a", "0", "r", "INITIALIZING", "n0")}, "n0", false},
		{"relocating source elsewhere counts as active", []responses.CatShardsResponse{sh("a", "0", "p", "RELOCATING", "n1 -> 10.0.0.2 id n2"), sh("a", "0", "r", "STARTED", "n0")}, "n0", false},
		{"candidate is relocating source of a sole copy", []responses.CatShardsResponse{sh("a", "0", "p", "RELOCATING", "n0 -> 10.0.0.2 id n2"), sh("a", "0", "r", "UNASSIGNED", "")}, "n0", true},
		{"master-only candidate holds nothing", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n0"), sh("a", "0", "r", "UNASSIGNED", "")}, "master-0", false},
		{"search replica elsewhere is not a copy", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n0"), sh("a", "0", "s", "STARTED", "n1"), sh("a", "0", "r", "UNASSIGNED", "")}, "n0", true},
		{"search replica with zero replicas behaves like green", []responses.CatShardsResponse{sh("a", "0", "p", "STARTED", "n0"), sh("a", "0", "s", "STARTED", "n1")}, "n0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, got := soleActiveCopyOnNode(tt.shards, tt.node); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func mockClient(t *testing.T, health, transientEnable, catShards string) (*OsClusterClient, *httpmock.MockTransport, *string) {
	tr := httpmock.NewMockTransport()
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		tr.RegisterResponder(m, `=~^http://os.test:9200/?$`, httpmock.NewStringResponder(200, `{"version":{"number":"2.0.0"}}`))
	}
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/health`, httpmock.NewStringResponder(200, health))
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/settings`, httpmock.NewStringResponder(200, `{"transient":{"cluster.routing.allocation.enable":"`+transientEnable+`"},"persistent":{}}`))
	put := new(string)
	tr.RegisterResponder(http.MethodPut, `=~/_cluster/settings`, func(r *http.Request) (*http.Response, error) {
		*put = "called"
		return httpmock.NewStringResponse(200, `{}`), nil
	})
	tr.RegisterResponder(http.MethodGet, `=~/_cat/shards`, httpmock.NewStringResponder(200, catShards))
	c, err := NewOsClusterClient("http://os.test:9200", "u", "p", WithTransport(tr))
	if err != nil {
		t.Fatal(err)
	}
	return c, tr, put
}

func TestCheckClusterStatusForRestartYellow(t *testing.T) {
	quiet := `{"status":"yellow","unassigned_shards":3,"number_of_data_nodes":3}`
	tests := []struct {
		name, health, enable string
		drain, want          bool
		wantPut              bool
	}{
		{"quiet yellow proceeds", quiet, "all", false, true, false},
		{"quiet yellow proceeds with drain", quiet, "all", true, true, false},
		{"own primaries throttle is lifted first", quiet, "primaries", false, false, true},
		{"throttle lifted with drain too", quiet, "none", true, false, true},
		{"missing data node waits", `{"status":"yellow","number_of_data_nodes":2}`, "all", false, false, false},
		{"initializing waits", `{"status":"yellow","number_of_data_nodes":3,"initializing_shards":1}`, "all", false, false, false},
		{"delayed allocation waits", `{"status":"yellow","number_of_data_nodes":3,"delayed_unassigned_shards":2}`, "all", false, false, false},
		{"in-flight fetch waits", `{"status":"yellow","number_of_data_nodes":3,"number_of_in_flight_fetch":1}`, "all", false, false, false},
		{"red never proceeds", `{"status":"red"}`, "all", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, tr, put := mockClient(t, tt.health, tt.enable, `[]`)
			got, msg, err := CheckClusterStatusForRestart(c, tt.drain, 3)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want || (*put != "") != tt.wantPut {
				t.Errorf("got %v (%q) put=%q, want %v put=%v", got, msg, *put, tt.want, tt.wantPut)
			}
			if n := tr.GetCallCountInfo()["GET =~/_cat/shards"]; n != 0 {
				t.Errorf("health gate must not list shards, got %d calls", n)
			}
		})
	}
}

func TestPreparePodForDeleteSoleCopy(t *testing.T) {
	sole := `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"UNASSIGNED","node":null}]`
	tests := []struct {
		name      string
		nodeCount int32
		want      bool
	}{
		{"multi-node blocks sole copy", 3, false},
		{"single data node is exempt", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _, put := mockClient(t, `{}`, "all", sole)
			got, err := PreparePodForDelete(c, logr.Discard(), "n0", false, tt.nodeCount)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want || (*put != "") != tt.want {
				t.Errorf("got %v put=%q, want %v", got, *put, tt.want)
			}
		})
	}
}

// With drain and 2 data nodes only system-index primaries are drained, so the
// sole-copy check must still guard the rest.
func TestPreparePodForDeleteDrainTwoNodes(t *testing.T) {
	system := `[{"index":".opendistro_security","shard":"0","prirep":"p","state":"STARTED","node":"n1"}]`
	tests := []struct {
		name, shards string
		want         bool
	}{
		{"sole copy blocks", `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"UNASSIGNED","node":null}]`, false},
		{"replica elsewhere proceeds", `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"STARTED","node":"n1"}]`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, tr, _ := mockClient(t, `{}`, "all", tt.shards)
			tr.RegisterResponder(http.MethodGet, `=~/_cat/indices/`, httpmock.NewStringResponder(404, `{}`))
			tr.RegisterResponder(http.MethodGet, `http://os.test:9200/_cat/indices/.opendistro_security`, httpmock.NewStringResponder(200, `[{"index":".opendistro_security"}]`))
			tr.RegisterResponder(http.MethodGet, `http://os.test:9200/_cat/shards/.opendistro_security`, httpmock.NewStringResponder(200, system))
			got, err := PreparePodForDelete(c, logr.Discard(), "n0", true, 2)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
