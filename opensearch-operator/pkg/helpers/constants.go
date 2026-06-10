package helpers

import (
	"os"
	"strconv"
)

const (
	DashboardConfigName          = "opensearch_dashboards.yml"
	DashboardChecksumName        = "checksum/dashboards.yml"
	ClusterLabel                 = "opensearch.org/opensearch-cluster"
	OldClusterLabel              = "opster.io/opensearch-cluster"
	JobLabel                     = "opensearch.org/opensearch-job"
	NodePoolLabel                = "opensearch.org/opensearch-nodepool"
	OsUserNameAnnotation         = "opensearchuser/name"
	OsUserNamespaceAnnotation    = "opensearchuser/namespace"
	DnsBaseEnvVariable           = "DNS_BASE"
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
