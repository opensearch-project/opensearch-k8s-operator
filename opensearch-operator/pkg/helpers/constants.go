package helpers

import "os"

const (
	DashboardConfigName       = "opensearch_dashboards.yml"
	DashboardChecksumName     = "checksum/dashboards.yml"
	OsUserNameAnnotation      = "opensearchuser/name"
	OsUserNamespaceAnnotation = "opensearchuser/namespace"
)

func ClusterDnsBase() string {
	env, found := os.LookupEnv("DNS_BASE")

	if !found {
		env = "cluster.local"
	}

	return env
}
