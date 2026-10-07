package controllers

import (
	"testing"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
)

func TestRecordDeprecation(t *testing.T) {
	t.Parallel()

	recorder := record.NewFakeRecorder(10)
	policy := &opensearchv1.OpenSearchISMPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: types.UID("test-record-deprecation"), Generation: 1},
	}
	expectEvents := func(step string, want int) {
		t.Helper()
		if got := len(recorder.Events); got != want {
			t.Fatalf("%s: got %d events, want %d", step, got, want)
		}
		for range want {
			if e := <-recorder.Events; e != "Warning Deprecated OpenSearchISMPolicy is deprecated and will be removed in v4 of the OpenSearch Kubernetes Operator" {
				t.Fatalf("%s: unexpected event %q", step, e)
			}
		}
	}

	recordDeprecation(recorder, policy, "OpenSearchISMPolicy")
	expectEvents("first reconcile", 1)

	recordDeprecation(recorder, policy, "OpenSearchISMPolicy")
	recordDeprecation(recorder, policy, "OpenSearchISMPolicy")
	expectEvents("requeues of the same generation", 0)

	policy.Generation = 2
	recordDeprecation(recorder, policy, "OpenSearchISMPolicy")
	expectEvents("spec change", 1)

	now := metav1.Now()
	policy.DeletionTimestamp = &now
	recordDeprecation(recorder, policy, "OpenSearchISMPolicy")
	expectEvents("deletion", 0)
	if _, ok := deprecationRecorded.Load(policy.UID); ok {
		t.Fatalf("deleted object still tracked")
	}
}
