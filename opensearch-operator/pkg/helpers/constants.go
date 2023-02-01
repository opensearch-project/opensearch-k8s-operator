package helpers

import (
	"os"
	"strconv"
)

const (
	DashboardConfigName       = "opensearch_dashboards.yml"
	DashboardChecksumName     = "checksum/dashboards.yml"
	OsUserNameAnnotation      = "opensearchuser/name"
	OsUserNamespaceAnnotation = "opensearchuser/namespace"
	DnsBaseEnvVariable        = "DNS_BASE"
	ClusterLabel              = "opster.io/opensearch-cluster"
	NodePoolLabel             = "opster.io/opensearch-nodepool"
	ParallelRecoveryEnabled   = "PARALLEL_RECOVERY_ENABLED"
)

func ClusterDnsBase() string {
	env, found := os.LookupEnv(DnsBaseEnvVariable)

	if !found || len(env) == 0 {
		env = "cluster.local"
	}

	return env
}

func ParallelRecoveryMode() bool {
	env, found := os.LookupEnv(ParallelRecoveryEnabled)

	if !found || len(env) == 0 {
		env = "true"
	}

	result, err := strconv.ParseBool(env)
	if err != nil {
		return true
	}
	return result
}
