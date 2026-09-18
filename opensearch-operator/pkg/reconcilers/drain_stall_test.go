package reconcilers

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/jarcoal/httpmock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
)

func TestEvaluateDrainStall(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	long := 15 * time.Minute
	started := func(at time.Time) string { return drainStartedConditionPrefix + at.UTC().Format(time.RFC3339) }

	tests := []struct {
		name        string
		current     []string
		pod         string
		activity    bool
		wantRelease bool
		wantStalled bool
		wantStart   time.Time
	}{
		{
			name:      "a first pass starts the clock",
			current:   nil,
			pod:       "n0",
			wantStart: now,
		},
		{
			name:      "shards are moving, so the clock restarts",
			current:   []string{"n0", started(now.Add(-time.Hour))},
			pod:       "n0",
			activity:  true,
			wantStart: now,
		},
		{
			name:      "a quiet pass inside the window keeps waiting",
			current:   []string{"n0", started(now.Add(-time.Minute))},
			pod:       "n0",
			wantStart: now.Add(-time.Minute),
		},
		{
			name:        "past the window the exclusion is released once",
			current:     []string{"n0", started(now.Add(-time.Hour))},
			pod:         "n0",
			wantRelease: true,
			wantStalled: true,
			wantStart:   now.Add(-time.Hour),
		},
		{
			name:        "already stalled, so nothing is released again",
			current:     []string{"n0", started(now.Add(-time.Hour)), drainStalledCondition},
			pod:         "n0",
			wantStalled: true,
			wantStart:   now.Add(-time.Hour),
		},
		{
			name:      "a different candidate starts its own clock",
			current:   []string{"n1", started(now.Add(-time.Hour)), drainStalledCondition},
			pod:       "n0",
			wantStart: now,
		},
		{
			name:      "movement clears a stall that was already recorded",
			current:   []string{"n0", started(now.Add(-time.Hour)), drainStalledCondition},
			pod:       "n0",
			activity:  true,
			wantStart: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateDrainStall(tt.current, tt.pod, tt.activity, now, long)
			if got.release != tt.wantRelease {
				t.Errorf("release: got %v, want %v", got.release, tt.wantRelease)
			}
			if got.stalled != tt.wantStalled {
				t.Errorf("stalled: got %v, want %v", got.stalled, tt.wantStalled)
			}
			if got.conditions[0] != tt.pod {
				t.Errorf("conditions must lead with the candidate, got %q", got.conditions[0])
			}
			at, ok := drainStartedAt(got.conditions)
			if !ok {
				t.Fatalf("conditions carry no start time: %v", got.conditions)
			}
			if !at.Equal(tt.wantStart) {
				t.Errorf("start: got %s, want %s", at, tt.wantStart)
			}
			if hasDrainStalledCondition(got.conditions) != tt.wantStalled {
				t.Errorf("stalled condition: got %v, want %v", hasDrainStalledCondition(got.conditions), tt.wantStalled)
			}
		})
	}
}

// The warning goes out with the release and not on every later pass.
func TestEvaluateDrainStallWarnsOnce(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	first := evaluateDrainStall([]string{"n0", drainStartedConditionPrefix + now.Add(-time.Hour).Format(time.RFC3339)}, "n0", false, now, 15*time.Minute)
	if !first.warn || !first.release {
		t.Fatalf("the crossing pass should warn and release, got %+v", first)
	}
	second := evaluateDrainStall(first.conditions, "n0", false, now.Add(time.Minute), 15*time.Minute)
	if second.warn || second.release {
		t.Errorf("a later pass must not warn or release again, got %+v", second)
	}
}

func drainOsClient(t *testing.T, health, excluded, catShards string) (*services.OsClusterClient, *[]string) {
	t.Helper()
	tr := httpmock.NewMockTransport()
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		tr.RegisterResponder(m, `=~^http://os.test:9200/?$`, httpmock.NewStringResponder(200, `{"version":{"number":"2.0.0"}}`))
	}
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/health`, httpmock.NewStringResponder(200, health))
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/settings`, httpmock.NewStringResponder(200,
		`{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"`+excluded+`"}}}}},"persistent":{}}`))
	puts := &[]string{}
	tr.RegisterResponder(http.MethodPut, `=~/_cluster/settings`, func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		*puts = append(*puts, string(b))
		return httpmock.NewStringResponse(200, `{}`), nil
	})
	tr.RegisterResponder(http.MethodGet, `=~/_cat/shards`, httpmock.NewStringResponder(200, catShards))
	c, err := services.NewOsClusterClient("http://os.test:9200", "u", "p", services.WithTransport(tr))
	if err != nil {
		t.Fatal(err)
	}
	return c, puts
}

const (
	quietYellow  = `{"status":"yellow","number_of_data_nodes":3,"unassigned_shards":1}`
	movingYellow = `{"status":"yellow","number_of_data_nodes":3,"relocating_shards":1}`
	shardOnN0    = `[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"}]`
	noShards     = `[]`
)

func stalledConditions(pod string, startedAt time.Time) []string {
	return drainConditions(pod, startedAt, true)
}

func TestStandStillOnStalledDrain(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		conditions []string
		health     string
		shards     string
		wantStand  bool
		wantClear  bool
	}{
		{"nothing recorded, so the ordinary path runs", nil, quietYellow, shardOnN0, false, false},
		{"recorded for another candidate", stalledConditions("n1", now), quietYellow, shardOnN0, false, false},
		{"waiting but not yet stalled", drainConditions("n0", now, false), quietYellow, shardOnN0, false, false},
		{"stalled and still quiet, so stand still", stalledConditions("n0", now.Add(-time.Hour)), quietYellow, shardOnN0, true, false},
		{"shards moving again clears it", stalledConditions("n0", now.Add(-time.Hour)), movingYellow, shardOnN0, false, true},
		{"the node emptied after all", stalledConditions("n0", now.Add(-time.Hour)), quietYellow, noShards, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, puts := drainOsClient(t, tt.health, "n0", tt.shards)
			stand, updated, err := standStillOnStalledDrain(c, "n0", tt.conditions, now)
			if err != nil {
				t.Fatal(err)
			}
			if stand != tt.wantStand {
				t.Errorf("standStill: got %v, want %v", stand, tt.wantStand)
			}
			if tt.wantClear && (updated == nil || hasDrainStalledCondition(updated)) {
				t.Errorf("the stall should have been cleared, got %v", updated)
			}
			if len(*puts) != 0 {
				t.Errorf("this decision must not write cluster settings, got %v", *puts)
			}
		})
	}
}

func clusterWith(componentStatuses ...opensearchv1.ComponentStatus) *opensearchv1.OpenSearchCluster {
	return &opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Status:     opensearchv1.ClusterStatus{ComponentsStatus: componentStatuses},
	}
}

func TestRecordUnfinishedDrain(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	t.Run("inside the window nothing is written to the cluster", func(t *testing.T) {
		c, puts := drainOsClient(t, quietYellow, "n0", shardOnN0)
		conditions, err := recordUnfinishedDrain(clusterWith(), c, nil, logr.Discard(), "n0", drainConditions("n0", now.Add(-time.Minute), false), now)
		if err != nil {
			t.Fatal(err)
		}
		if hasDrainStalledCondition(conditions) {
			t.Errorf("not stalled yet, got %v", conditions)
		}
		if len(*puts) != 0 {
			t.Errorf("no settings write expected, got %v", *puts)
		}
	})

	t.Run("past the window the exclusion is released once", func(t *testing.T) {
		c, puts := drainOsClient(t, quietYellow, "n0", shardOnN0)
		conditions, err := recordUnfinishedDrain(clusterWith(), c, nil, logr.Discard(), "n0", drainConditions("n0", now.Add(-time.Hour), false), now)
		if err != nil {
			t.Fatal(err)
		}
		if !hasDrainStalledCondition(conditions) {
			t.Fatalf("expected a stalled condition, got %v", conditions)
		}
		if len(*puts) != 1 {
			t.Fatalf("expected exactly one settings write, got %v", *puts)
		}
		if strings.Contains((*puts)[0], `"n0"`) {
			t.Errorf("the write should clear the exclusion, got %s", (*puts)[0])
		}

		// A second pass on the same state writes nothing more.
		c2, puts2 := drainOsClient(t, quietYellow, "", shardOnN0)
		if _, err := recordUnfinishedDrain(clusterWith(), c2, nil, logr.Discard(), "n0", conditions, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if len(*puts2) != 0 {
			t.Errorf("the release must happen once, got %v", *puts2)
		}
	})

	t.Run("a scaler drain owns the exclude list", func(t *testing.T) {
		c, puts := drainOsClient(t, quietYellow, "n0", shardOnN0)
		instance := clusterWith(opensearchv1.ComponentStatus{Component: "Scaler", Status: "Drained"})
		if _, err := recordUnfinishedDrain(instance, c, nil, logr.Discard(), "n0", drainConditions("n0", now.Add(-time.Hour), false), now); err != nil {
			t.Fatal(err)
		}
		if len(*puts) != 0 {
			t.Errorf("the scaler's exclusion must not be touched, got %v", *puts)
		}
	})

	t.Run("movement restarts the clock", func(t *testing.T) {
		c, puts := drainOsClient(t, movingYellow, "n0", shardOnN0)
		conditions, err := recordUnfinishedDrain(clusterWith(), c, nil, logr.Discard(), "n0", drainConditions("n0", now.Add(-time.Hour), false), now)
		if err != nil {
			t.Fatal(err)
		}
		if hasDrainStalledCondition(conditions) {
			t.Errorf("a moving drain is not stalled, got %v", conditions)
		}
		at, _ := drainStartedAt(conditions)
		if !at.Equal(now) {
			t.Errorf("clock should restart at now, got %s", at)
		}
		if len(*puts) != 0 {
			t.Errorf("no settings write expected, got %v", *puts)
		}
	})
}

// The shape the whole change exists for: a drain that can never finish must cost
// one exclusion and one withdrawal, and then stop touching the cluster. The
// exclude list is live here, the way OpenSearch keeps it, so a pass that re-applies
// the exclusion shows up as another write.
func TestStalledDrainSettlesAfterOneExclusionAndOneWithdrawal(t *testing.T) {
	start := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	excluded := ""
	writes := 0

	tr := httpmock.NewMockTransport()
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		tr.RegisterResponder(m, `=~^http://os.test:9200/?$`, httpmock.NewStringResponder(200, `{"version":{"number":"2.0.0"}}`))
	}
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/health`, httpmock.NewStringResponder(200, quietYellow))
	tr.RegisterResponder(http.MethodGet, `=~/_cluster/settings`, func(*http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(200,
			`{"transient":{"cluster":{"routing":{"allocation":{"enable":"all","exclude":{"_name":"`+excluded+`"}}}}},"persistent":{}}`), nil
	})
	tr.RegisterResponder(http.MethodPut, `=~/_cluster/settings`, func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		writes++
		if strings.Contains(string(body), `"_name":"n0"`) {
			excluded = "n0"
		} else if strings.Contains(string(body), `"exclude"`) {
			excluded = ""
		}
		return httpmock.NewStringResponse(200, `{}`), nil
	})
	// The node holds a shard whose replica can never be assigned, so it never empties.
	tr.RegisterResponder(http.MethodGet, `=~/_cat/shards`, httpmock.NewStringResponder(200,
		`[{"index":"a","shard":"0","prirep":"p","state":"STARTED","node":"n0"},{"index":"a","shard":"0","prirep":"r","state":"UNASSIGNED","node":null}]`))
	tr.RegisterResponder(http.MethodGet, `=~/_cat/indices`, httpmock.NewStringResponder(200, `[]`))

	c, err := services.NewOsClusterClient("http://os.test:9200", "u", "p", services.WithTransport(tr))
	if err != nil {
		t.Fatal(err)
	}

	instance := clusterWith()
	conditions := []string(nil)
	excludedSeen := 0
	for pass := 0; pass < 10; pass++ {
		now := start.Add(time.Duration(pass) * 10 * time.Minute)

		standStill, updated, err := standStillOnStalledDrain(c, "n0", conditions, now)
		if err != nil {
			t.Fatal(err)
		}
		if updated != nil {
			conditions = updated
		}
		if standStill {
			if excluded != "" {
				t.Fatalf("pass %d stands still while the node is still excluded", pass)
			}
			continue
		}

		ready, err := services.PreparePodForDelete(c, logr.Discard(), "n0", true, 3)
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			t.Fatalf("pass %d: the node still holds a shard, it is not ready", pass)
		}
		if excluded == "n0" {
			excludedSeen++
		}
		conditions, err = recordUnfinishedDrain(instance, c, nil, logr.Discard(), "n0", conditions, now)
		if err != nil {
			t.Fatal(err)
		}
	}

	if writes != 2 {
		t.Errorf("expected one exclusion and one withdrawal, got %d writes", writes)
	}
	if excluded != "" {
		t.Errorf("the node should not be left excluded, exclude list holds %q", excluded)
	}
	if !hasDrainStalledCondition(conditions) {
		t.Errorf("the stall should be recorded, got %v", conditions)
	}
	if excludedSeen == 0 {
		t.Errorf("the drain should have been attempted at least once")
	}
}
