package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// TLSCertExpiryDays tracks the number of days until TLS certificate expiration
	TLSCertExpiryDays = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "opensearch_tls_certificate_expiry_days",
			Help: "Days until TLS certificate expiration",
		},
		[]string{"cluster", "namespace", "certificate_type"},
	)
)

func init() {
	// Register metrics with the global prometheus registry
	metrics.Registry.MustRegister(TLSCertExpiryDays)
}
