package helpers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = DescribeTable("ClusterHealth metric value mapping",
	func(health opensearchv1.OpenSearchHealth, expected float64) {
		instance := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "health-metric-test", Namespace: "default"},
		}
		DeferCleanup(DeleteClusterMetrics, instance.Namespace, instance.Name)

		UpdateClusterInfo(instance, health, responses.ClusterHealthResponse{})

		gauge := ClusterHealth.With(prometheus.Labels{"namespace": instance.Namespace, "opensearch_cluster": instance.Name})
		Expect(testutil.ToFloat64(gauge)).To(Equal(expected))
	},
	// Keep in sync with the ClusterHealth Help text and docs/designs/monitoring.md.
	Entry("green is 0", opensearchv1.OpenSearchGreenHealth, float64(0)),
	Entry("yellow is 1", opensearchv1.OpenSearchYellowHealth, float64(1)),
	Entry("red is 2", opensearchv1.OpenSearchRedHealth, float64(2)),
	Entry("unknown is -1", opensearchv1.OpenSearchUnknownHealth, float64(-1)),
)
