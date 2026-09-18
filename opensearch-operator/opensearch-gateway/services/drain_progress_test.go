package services

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/jarcoal/httpmock"
)

// drainMockClient serves the endpoints a drain decision reads: health, cluster
// settings (with the exclude list), the shard table, and one allocation
// explanation per shard, keyed by "<index>/<shard>".
func drainMockClient(t *testing.T, health, excluded, catShards string, explains map[string]string) (*OsClusterClient, *httpmock.MockTransport, *[]string) {
	t.Helper()
	tr := httpmock.NewMockTransport()
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		tr.RegisterResponder(m, `=~^http://os.test:9200/?$`, httpmock.NewStringResponder(200, `{"version":{"number":"2.0.0"}}`))
	}
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/health`, httpmock.NewStringResponder(200, health))
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/settings`, httpmock.NewStringResponder(200,
		`{"transient":{"cluster":{"routing":{"allocation":{"enable":"all","exclude":{"_name":"`+excluded+`"}}}}},"persistent":{}}`))
	puts := &[]string{}
	tr.RegisterResponder(http.MethodPut, `=~/_cluster/settings`, func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		*puts = append(*puts, string(b))
		return httpmock.NewStringResponse(200, `{}`), nil
	})
	tr.RegisterResponder(http.MethodGet, `=~/_cat/shards`, httpmock.NewStringResponder(200, catShards))
	tr.RegisterResponder(http.MethodGet, `=~/_cat/indices`, httpmock.NewStringResponder(200, `[]`))
	tr.RegisterResponder(http.MethodPost, `=~/_cluster/allocation/explain`, func(r *http.Request) (*http.Response, error) {
		var req struct {
			Index string `json:"index"`
			Shard int    `json:"shard"`
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &req); err != nil {
			return httpmock.NewStringResponse(400, `{}`), nil
		}
		body, ok := explains[req.Index]
		if !ok {
			return httpmock.NewStringResponse(404, `{}`), nil
		}
		return httpmock.NewStringResponse(200, body), nil
	})
	c, err := NewOsClusterClient("http://os.test:9200", "u", "p", WithTransport(tr))
	if err != nil {
		t.Fatal(err)
	}
	return c, tr, puts
}

func explainBody(canMove string) string {
	return `{"index":"a","shard":0,"primary":true,"current_state":"started","can_remain_on_current_node":"no","can_move_to_other_node":"` + canMove + `","node_allocation_decisions":[]}`
}

func TestDrainCanProceed(t *testing.T) {
	onNode := `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"}]`
	tests := []struct {
		name     string
		shards   string
		explains map[string]string
		want     bool
	}{
		{"a shard that can move keeps the drain alive", onNode, map[string]string{"a": explainBody("yes")}, true},
		{"a shard with nowhere to go stops it", onNode, map[string]string{"a": explainBody("no")}, false},
		{
			"one immovable shard is enough to stop it",
			`[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"b","shard":"0","prirep":"p","state":"STARTED","node":"n0"}]`,
			map[string]string{"a": explainBody("yes"), "b": explainBody("no")},
			false,
		},
		{"an empty node has nothing to block it", `[]`, nil, true},
		{"shards on other nodes are not this node's problem",
			`[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n1"}]`,
			map[string]string{"a": explainBody("no")}, true},
		{"a relocating shard is already moving",
			`[{"index":"a","shard":"0","prirep":"p","state":"RELOCATING","node":"n0 -> 10.0.0.2 id n2"}]`,
			map[string]string{"a": explainBody("no")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _, _ := drainMockClient(t, `{"status":"yellow"}`, "", tt.shards, tt.explains)
			got, _, err := drainCanProceed(c, "n0")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// The node holds a shard nothing else can take, so the drain will not finish.
// The exclusion has to be released rather than held for as long as the restart
// is stuck, and it must not be re-applied on the next pass either.
func TestPreparePodForDeleteReleasesAnUnproductiveExclusion(t *testing.T) {
	stuck := `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"UNASSIGNED","node":null}]`
	quietYellow := `{"status":"yellow","number_of_data_nodes":3,"unassigned_shards":1}`

	t.Run("an exclusion already in place is withdrawn", func(t *testing.T) {
		c, _, puts := drainMockClient(t, quietYellow, "n0", stuck, map[string]string{"a": explainBody("no")})
		ready, err := PreparePodForDelete(c, logr.Discard(), "n0", true, 3)
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			t.Fatal("the pod is not safe to delete, it still holds a shard")
		}
		if len(*puts) != 1 {
			t.Fatalf("expected exactly one settings write, got %d: %v", len(*puts), *puts)
		}
		if strings.Contains((*puts)[0], `"n0"`) {
			t.Errorf("the write should clear the exclusion, got %s", (*puts)[0])
		}
	})

	t.Run("no exclusion is applied in the first place", func(t *testing.T) {
		c, _, puts := drainMockClient(t, quietYellow, "", stuck, map[string]string{"a": explainBody("no")})
		ready, err := PreparePodForDelete(c, logr.Discard(), "n0", true, 3)
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			t.Fatal("the pod is not safe to delete, it still holds a shard")
		}
		if len(*puts) != 0 {
			t.Errorf("nothing should be written when the node was never excluded, got %v", *puts)
		}
	})
}

// A drain that can still make progress keeps the behaviour it had: exclude the
// node and wait for it to empty.
func TestPreparePodForDeleteStillDrainsWhenItCan(t *testing.T) {
	movable := `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"STARTED","node":"n1"}]`
	c, _, puts := drainMockClient(t, `{"status":"green","number_of_data_nodes":3}`, "", movable, map[string]string{"a": explainBody("yes")})
	ready, err := PreparePodForDelete(c, logr.Discard(), "n0", true, 3)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("the node is not empty yet")
	}
	if len(*puts) != 1 || !strings.Contains((*puts)[0], `n0`) {
		t.Fatalf("expected the node to be excluded, writes were %v", *puts)
	}
}

// While shards are actually moving there is nothing to decide, and the
// allocation explanation must not be asked for at all.
func TestPreparePodForDeleteDoesNotExplainWhileShardsMove(t *testing.T) {
	moving := `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"UNASSIGNED","node":null}]`
	c, tr, _ := drainMockClient(t, `{"status":"yellow","number_of_data_nodes":3,"relocating_shards":1}`, "n0", moving, map[string]string{"a": explainBody("no")})
	if _, err := PreparePodForDelete(c, logr.Discard(), "n0", true, 3); err != nil {
		t.Fatal(err)
	}
	if n := tr.GetCallCountInfo()["POST =~/_cluster/allocation/explain"]; n != 0 {
		t.Errorf("explain must not be called while the allocator is busy, got %d calls", n)
	}
}
