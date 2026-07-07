package v1

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// Regression: plain bool + omitempty omitted explicit false and broke GitOps sync (issue #1172).
func TestOpensearchSnapshotPolicySpec_JSONRetainsExplicitFalseBooleans(t *testing.T) {
	spec := OpensearchSnapshotPolicySpec{
		OpensearchRef: corev1.LocalObjectReference{Name: "cluster"},
		PolicyName:    "policy",
		SnapshotConfig: SnapshotConfig{
			Repository:         "repo",
			IgnoreUnavailable:  ptr.To(false),
			IncludeGlobalState: ptr.To(false),
			Partial:            ptr.To(false),
		},
		Creation: SnapshotCreation{
			Schedule: CronSchedule{
				Cron: CronExpression{
					Expression: "0 0 * * *",
					Timezone:   "UTC",
				},
			},
		},
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, key := range []string{`"ignoreUnavailable":false`, `"includeGlobalState":false`, `"partial":false`} {
		if !strings.Contains(s, key) {
			t.Fatalf("expected JSON to contain %q, got: %s", key, s)
		}
	}
}

func TestOpensearchSnapshotPolicySpec_JSONOmitsUnsetSnapshotBooleans(t *testing.T) {
	spec := OpensearchSnapshotPolicySpec{
		OpensearchRef: corev1.LocalObjectReference{Name: "cluster"},
		PolicyName:    "policy",
		SnapshotConfig: SnapshotConfig{
			Repository: "repo",
		},
		Creation: SnapshotCreation{
			Schedule: CronSchedule{
				Cron: CronExpression{
					Expression: "0 0 * * *",
					Timezone:   "UTC",
				},
			},
		},
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, key := range []string{"ignoreUnavailable", "includeGlobalState", "partial"} {
		if strings.Contains(s, key) {
			t.Fatalf("expected JSON to omit %q when unset, got: %s", key, s)
		}
	}
}
