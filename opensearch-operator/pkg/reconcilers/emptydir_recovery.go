package reconcilers

import (
	"time"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
)

const (
	emptyDirRecoveryComponent     = "EmptyDirRecovery"
	emptyDirRecoveryStatusPending = "Pending"
	emptyDirRecoveryGracePeriod   = 5 * time.Minute

	// emptyDirPodUIDComponent stores, per pod name (in Description), the UID of the
	// last pod observed Ready for that name. A StatefulSet recreates pods with the
	// same name but a new UID, so this is what lets us tell "the pod object that
	// held the emptyDir data is gone" apart from "the pod object is merely not
	// ready right now" (see #1526 vs #1455).
	emptyDirPodUIDComponent = "EmptyDirPodUID"
)

type emptyDirPodStats struct {
	existingDataPods   int32
	totalDataPods      int32
	existingMasterPods int32
	totalMasterPods    int32
}

// emptyDirDataLossSuspected reports whether pods are actually missing, not merely not-ready.
// emptyDir volumes survive in-place pod restarts while the pod object still exists.
func emptyDirDataLossSuspected(stats emptyDirPodStats) bool {
	dataNodesMissing := stats.totalDataPods > 0 && stats.existingDataPods == 0
	mastersLostQuorum := stats.totalMasterPods > 0 && stats.existingMasterPods < (stats.totalMasterPods+1)/2
	return dataNodesMissing || mastersLostQuorum
}

func emptyDirRecoveryFirstObserved(components []opensearchv1.ComponentStatus) (time.Time, bool) {
	for _, component := range components {
		if component.Component != emptyDirRecoveryComponent {
			continue
		}
		firstObserved, err := time.Parse(time.RFC3339, component.Description)
		if err != nil {
			return time.Time{}, false
		}
		return firstObserved, true
	}
	return time.Time{}, false
}

// recordedEmptyDirPodUID returns the last known-good UID recorded for the pod name.
func recordedEmptyDirPodUID(components []opensearchv1.ComponentStatus, podName string) (string, bool) {
	for _, component := range components {
		if component.Component == emptyDirPodUIDComponent && component.Description == podName {
			return component.Status, true
		}
	}
	return "", false
}

// classifyEmptyDirPods reports how many of the given pods still hold their original
// emptyDir data, plus any pod UID records that need to be (re)persisted.
//
// A pod counts as existing if it is Ready (its emptyDir is known-good; its UID is
// (re)recorded), or if it is not Ready but its UID matches the last recorded
// known-good UID for its name (a readiness blip or crash loop on the same pod that
// never lost its emptyDir, see #1455). A pod that is not Ready and whose UID was
// never recorded as Ready (recreated after a force-delete, eviction, or node loss,
// see #1526) does not count as existing.
func classifyEmptyDirPods(pods []corev1.Pod, recorded []opensearchv1.ComponentStatus) (existing int, updates []opensearchv1.ComponentStatus) {
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue
		}

		uid := string(pod.UID)
		recordedUID, hasRecord := recordedEmptyDirPodUID(recorded, pod.Name)

		if helpers.IsPodReady(pod) {
			existing++
			if !hasRecord || recordedUID != uid {
				updates = append(updates, opensearchv1.ComponentStatus{
					Component:   emptyDirPodUIDComponent,
					Description: pod.Name,
					Status:      uid,
				})
			}
			continue
		}

		if hasRecord && recordedUID == uid {
			existing++
		}
	}
	return existing, updates
}

// upsertComponentStatus replaces the component status matching update's Component and
// Description, or appends it if none is found.
func upsertComponentStatus(components []opensearchv1.ComponentStatus, update opensearchv1.ComponentStatus) []opensearchv1.ComponentStatus {
	for i, component := range components {
		if component.Component == update.Component && component.Description == update.Description {
			components[i] = update
			return components
		}
	}
	return append(components, update)
}
