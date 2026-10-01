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
		Expect(gauge.Desc().String()).To(ContainSubstring(
			"0=green, 1=yellow, 2=red, -1=unknown",
		))
	},
	// Keep in sync with the ClusterHealth Help text and docs/designs/monitoring.md.
	Entry("green is 0", opensearchv1.OpenSearchGreenHealth, float64(0)),
	Entry("yellow is 1", opensearchv1.OpenSearchYellowHealth, float64(1)),
	Entry("red is 2", opensearchv1.OpenSearchRedHealth, float64(2)),
	Entry("unknown is -1", opensearchv1.OpenSearchUnknownHealth, float64(-1)),
)

var _ = Describe("DeleteClusterMetrics", func() {
	It("removes every series for the cluster, including a previous info version", func() {
		instance := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "health-metric-cleanup", Namespace: "default"},
		}
		DeferCleanup(DeleteClusterMetrics, instance.Namespace, instance.Name)

		instance.Status.Version = "1.0.0"
		UpdateClusterInfo(instance, opensearchv1.OpenSearchGreenHealth, responses.ClusterHealthResponse{ActiveShards: 1})
		instance.Status.Version = "2.0.0"
		UpdateClusterInfo(instance, opensearchv1.OpenSearchYellowHealth, responses.ClusterHealthResponse{ActiveShards: 3})

		TlsCertificateDaysRemaining.With(prometheus.Labels{
			"namespace": instance.Namespace, "opensearch_cluster": instance.Name, "interface": "http", "node": "data-0",
		}).Set(10)

		Expect(testutil.CollectAndCount(ClusterInfo)).To(Equal(1))
		Expect(testutil.ToFloat64(ClusterInfo.With(prometheus.Labels{
			"namespace": instance.Namespace, "opensearch_cluster": instance.Name, "version": "2.0.0",
		}))).To(Equal(float64(1)))
		Expect(testutil.CollectAndCount(ClusterShards)).To(Equal(4))
		Expect(testutil.CollectAndCount(ClusterHealth)).To(Equal(1))
		Expect(testutil.CollectAndCount(TlsCertificateDaysRemaining)).To(Equal(1))

		DeleteClusterMetrics(instance.Namespace, instance.Name)

		Expect(testutil.CollectAndCount(ClusterInfo)).To(Equal(0))
		Expect(testutil.CollectAndCount(ClusterShards)).To(Equal(0))
		Expect(testutil.CollectAndCount(ClusterHealth)).To(Equal(0))
		Expect(testutil.CollectAndCount(TlsCertificateDaysRemaining)).To(Equal(0))
	})
})
