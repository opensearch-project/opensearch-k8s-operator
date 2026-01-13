package helpers

import (
	"embed"
	"fmt"

	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
)

var defaultSecurityConfigFilenames = []string{
	"internal_users.yml",
}

//go:embed securityconfigdefaults/*
var defaultSecurityConfigFS embed.FS

func defaultSecurityconfigData() (map[string][]byte, error) {
	data := make(map[string][]byte, len(defaultSecurityConfigFilenames))
	for _, file := range defaultSecurityConfigFilenames {
		content, err := defaultSecurityConfigFS.ReadFile(fmt.Sprintf("securityconfigdefaults/%s", file))
		if err != nil {
			return nil, err
		}
		data[file] = content
	}
	return data, nil
}

func GeneratedSecurityConfigSecretName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-security-config-generated", cr.Name)
}

func GeneratedAdminCredentialsSecretName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-admin-password", cr.Name)
}

func GeneratedDashboardsCredentialsSecretName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-dashboards-password", cr.Name)
}
