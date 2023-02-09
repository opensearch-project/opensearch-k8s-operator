package helpers

import (
	"os"
	"strconv"
)

const (
	DashboardConfigName          = "opensearch_dashboards.yml"
	DashboardChecksumName        = "checksum/dashboards.yml"
	ClusterLabel                 = "opster.io/opensearch-cluster"
	NodePoolLabel                = "opster.io/opensearch-nodepool"
	OsUserNameAnnotation         = "opensearchuser/name"
	OsUserNamespaceAnnotation    = "opensearchuser/namespace"
	DnsBaseEnvVariable           = "DNS_BASE"
	ParallelRecoveryEnabled      = "PARALLEL_RECOVERY_ENABLED"
	SkipInitContainerEnvVariable = "SKIP_INIT_CONTAINER"
)

func SkipInitContainer() bool {
	env, found := os.LookupEnv(SkipInitContainerEnvVariable)

	if !found || len(env) == 0 {
		return false
	}
	ok, err := strconv.ParseBool(env)
	if err != nil {
		return false
	}
	return ok
}

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
