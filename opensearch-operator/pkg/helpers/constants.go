package helpers

import "os"

const (
	DashboardConfigName   = "opensearch_dashboards.yml"
	DashboardChecksumName = "checksum/dashboards.yml"
	ClusterLabels         = "opster.io/opensearch-cluster"
	//NodePoolLabels                = "opster.io/opensearch-nodepool"
	OsUserNameAnnotation      = "opensearchuser/name"
	OsUserNamespaceAnnotation = "opensearchuser/namespace"
	DnsBaseEnvVariable        = "DNS_BASE"
	//ParallelRecoveryEnabled      = "PARALLEL_RECOVERY_ENABLED"
	//SkipInitContainerEnvVariable = "SKIP_INIT_CONTAINER"
)

func ClusterDnsBase() string {
	env, found := os.LookupEnv(DnsBaseEnvVariable)

	if !found || len(env) == 0 {
		env = "cluster.local"
	}

	return env
}
