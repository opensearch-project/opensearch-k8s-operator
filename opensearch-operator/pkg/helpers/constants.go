package helpers

import "os"

const (
	DashboardConfigName       = "opensearch_dashboards.yml"
	DashboardChecksumName     = "checksum/dashboards.yml"
	ClusterLabel              = "opster.io/opensearch-cluster"
	NodePoolLabel             = "opster.io/opensearch-nodepool"
	OsUserNameAnnotation      = "opensearchuser/name"
	OsUserNamespaceAnnotation = "opensearchuser/namespace"
	DnsBaseEnvVariable        = "DNS_BASE"
)

func ClusterDnsBase() string {
	env, found := os.LookupEnv(DnsBaseEnvVariable)

	if !found || len(env) == 0 {
		env = "cluster.local"
	}

	return env
}
