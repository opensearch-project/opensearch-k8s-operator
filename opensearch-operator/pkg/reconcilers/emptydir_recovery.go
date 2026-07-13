package reconcilers

import (
	"time"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
)

const (
	emptyDirRecoveryComponent      = "EmptyDirRecovery"
	emptyDirRecoveryStatusPending  = "Pending"
	emptyDirRecoveryGracePeriod    = 5 * time.Minute
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
