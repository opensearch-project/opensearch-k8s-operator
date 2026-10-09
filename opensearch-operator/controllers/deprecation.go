package controllers

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const reasonDeprecated = "Deprecated"

// The event recorder rate-limits Warning events per object regardless of
// reason, so emitting this on every requeue would starve the reconcilers' own
// Warning events. Record it once per object generation instead.
var deprecationRecorded sync.Map // UID -> generation (int64)

func recordDeprecation(recorder record.EventRecorder, obj client.Object, kind string) {
	uid := obj.GetUID()
	if !obj.GetDeletionTimestamp().IsZero() {
		deprecationRecorded.Delete(uid)
		return
	}
	generation := obj.GetGeneration()
	if prev, seen := deprecationRecorded.Swap(uid, generation); seen && prev.(int64) == generation {
		return
	}
	recorder.Event(obj, corev1.EventTypeWarning, reasonDeprecated,
		fmt.Sprintf("%s is deprecated and will be removed in v4 of the OpenSearch Kubernetes Operator", kind))
}
