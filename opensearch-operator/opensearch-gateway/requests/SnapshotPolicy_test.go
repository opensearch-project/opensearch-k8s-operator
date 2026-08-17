package requests

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/utils/ptr"
)

func TestSnapshotConfig_JSONIncludesFalseBooleans(t *testing.T) {
	cfg := SnapshotConfig{
		Repository:         "repo",
		IgnoreUnavailable:  ptr.To(false),
		IncludeGlobalState: ptr.To(false),
		Partial:            ptr.To(false),
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, key := range []string{`"ignore_unavailable":false`, `"include_global_state":false`, `"partial":false`} {
		if !strings.Contains(s, key) {
			t.Fatalf("expected JSON to contain %q, got: %s", key, s)
		}
	}
}

func TestSnapshotConfig_JSONOmitsNilBooleans(t *testing.T) {
	cfg := SnapshotConfig{Repository: "repo"}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, key := range []string{"ignore_unavailable", "include_global_state", "partial"} {
		if strings.Contains(s, key) {
			t.Fatalf("expected JSON to omit %q when nil, got: %s", key, s)
		}
	}
}
