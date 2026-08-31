package helpers

import (
	"os"
	"strconv"
	"strings"
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
	PodNamespaceEnvVariable      = "POD_NAMESPACE"
	serviceAccountNamespaceFile  = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
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

// OperatorNamespace returns the namespace the operator is running in.
// It prefers POD_NAMESPACE and falls back to the serviceaccount namespace file.
// An empty string means the namespace could not be determined.
func OperatorNamespace() string {
	if ns := os.Getenv(PodNamespaceEnvVariable); ns != "" {
		return ns
	}
	data, err := os.ReadFile(serviceAccountNamespaceFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
