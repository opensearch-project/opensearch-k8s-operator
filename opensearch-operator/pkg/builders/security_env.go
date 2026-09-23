package builders

import (
	"slices"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
)

// withSecurityConfigEnv returns spec.security.config.env followed by env. A variable in env takes
// precedence over a security config variable with the same name.
func withSecurityConfigEnv(cr *opensearchv1.OpenSearchCluster, env []corev1.EnvVar) []corev1.EnvVar {
	var result []corev1.EnvVar
	for _, v := range cr.Spec.Security.GetConfig().GetEnv() {
		if !slices.ContainsFunc(env, func(e corev1.EnvVar) bool { return e.Name == v.Name }) {
			result = append(result, v)
		}
	}
	return append(result, env...)
}
