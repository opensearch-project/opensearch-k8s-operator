package helpers

import (
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// see https://book.kubebuilder.io/reference/metrics#publishing-additional-metrics

const (
	clusterMetricsPrefix = "opensearch_operator_cluster_"
)

var (
	TlsCertificateDaysRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: clusterMetricsPrefix + "tls_certificate_remaining_days",
			Help: "Days until the certificate expires.",
		}, []string{
			"namespace", "opensearch_cluster", "interface", "node",
		})
	ClusterInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: clusterMetricsPrefix + "info",
			Help: "An info metric containing the cluster name, namespace, and version.",
		}, []string{
			"namespace", "opensearch_cluster", "version",
		})
	ClusterHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: clusterMetricsPrefix + "health",
			Help: "Health status of the cluster. 0=red, 1=yellow, 2=green, -1=unknown",
		}, []string{
			"namespace", "opensearch_cluster",
		})
	ClusterShards = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: clusterMetricsPrefix + "shards",
			Help: "The number of shards in the cluster.",
		}, []string{
			"namespace", "opensearch_cluster", "status",
		})
)

func RegisterMetrics() {
	metrics.Registry.MustRegister(TlsCertificateDaysRemaining, ClusterInfo, ClusterHealth, ClusterShards)
}

func DeleteClusterMetrics(namespace string, clusterName string) {
	TlsCertificateDaysRemaining.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ClusterInfo.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ClusterHealth.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ClusterShards.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
}

func UpdateClusterInfo(instance *opsterv1.OpenSearchCluster, health opsterv1.OpenSearchHealth, healthResponse responses.ClusterHealthResponse) {
	namespace := instance.Namespace
	clusterName := instance.Name

	// Delete the old version in case it has changed
	ClusterInfo.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ClusterInfo.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName, "version": instance.Status.Version}).Set(1)

	var value float64
	switch health {
	case opsterv1.OpenSearchRedHealth:
		value = 2
	case opsterv1.OpenSearchYellowHealth:
		value = 1
	case opsterv1.OpenSearchGreenHealth:
		value = 0
	default:
		value = -1
	}
	ClusterHealth.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName}).Set(value)

	if health != opsterv1.OpenSearchUnknownHealth {
		ClusterShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName, "status": "active"}).Set(float64(healthResponse.ActiveShards))
		ClusterShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName, "status": "relocating"}).Set(float64(healthResponse.RelocatingShards))
		ClusterShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName, "status": "initializing"}).Set(float64(healthResponse.InitializingShards))
		ClusterShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName, "status": "unassigned"}).Set(float64(healthResponse.UnassignedShards))
	}
}
