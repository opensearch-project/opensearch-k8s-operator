package helpers

import (
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// see https://book.kubebuilder.io/reference/metrics#publishing-additional-metrics

var (
	TlsCertificateDaysRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_tls_certificate_remaining_days",
			Help: "Days until the certificate expires.",
		}, []string{
			"namespace", "opensearch_cluster", "interface", "node",
		})
	ClusterInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_cluster_info",
			Help: "",
		}, []string{
			"namespace", "opensearch_cluster", "version",
		})
	ClusterHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_cluster_health",
			Help: "Health status of the cluster. 0=red, 1=yellow, 2=green, -1=unknown",
		}, []string{
			"namespace", "opensearch_cluster",
		})
	ActiveShards = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_cluster_shards_active",
			Help: "The number of active primary and replica shards.",
		}, []string{
			"namespace", "opensearch_cluster",
		})
	RelocatingShards = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_cluster_shards_relocating",
			Help: "The number of shards that are currently relocating.",
		}, []string{
			"namespace", "opensearch_cluster",
		})
	InitializingShards = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_cluster_shards_initializing",
			Help: "The number of shards that are currently initializing.",
		}, []string{
			"namespace", "opensearch_cluster",
		})
	UnassignedShards = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_cluster_shards_unassigned",
			Help: "The number of shards that are currently unassigned.",
		}, []string{
			"namespace", "opensearch_cluster",
		})
)

func RegisterMetrics() {
	metrics.Registry.MustRegister(TlsCertificateDaysRemaining, ClusterInfo, ClusterHealth, ActiveShards, RelocatingShards, InitializingShards, UnassignedShards)
}

func DeleteClusterMetrics(namespace string, clusterName string) {
	TlsCertificateDaysRemaining.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ClusterInfo.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ClusterHealth.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	ActiveShards.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	RelocatingShards.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	InitializingShards.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
	UnassignedShards.Delete(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName})
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
		value = 0
	case opsterv1.OpenSearchYellowHealth:
		value = 1
	case opsterv1.OpenSearchGreenHealth:
		value = 2
	default:
		value = -1
	}
	ClusterHealth.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName}).Set(value)

	if health != opsterv1.OpenSearchUnknownHealth {
		ActiveShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName}).Set(float64(healthResponse.ActiveShards))
		RelocatingShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName}).Set(float64(healthResponse.RelocatingShards))
		InitializingShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName}).Set(float64(healthResponse.InitializingShards))
		UnassignedShards.With(prometheus.Labels{"namespace": namespace, "opensearch_cluster": clusterName}).Set(float64(healthResponse.UnassignedShards))
	}
}
