package builders

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	opsterv1 "opensearch.opster.io/api/v1"
)

func ClusterDescWithversion(version string) opsterv1.OpenSearchCluster {
	return opsterv1.OpenSearchCluster{
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: version,
			},
		},
	}
}

func ClusterDescWithKeystoreSecret(secretName string) opsterv1.OpenSearchCluster {
	return opsterv1.OpenSearchCluster{
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				KeystoreValues: []opsterv1.KeystoreValue{
					{
						Secret: corev1.LocalObjectReference{
							Name: secretName,
						},
					},
				},
			},
		},
	}
}

var _ = Describe("Builders", func() {

	When("Constructing a STS for a NodePool", func() {
		It("should only use valid roles", func() {
			var clusterObject = ClusterDescWithversion("2.2.1")
			var nodePool = opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "cluster_manager,ingest",
			}))
		})
		It("should convert the master role", func() {
			var clusterObject = ClusterDescWithversion("2.2.1")
			var nodePool = opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"master"},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "cluster_manager",
			}))
		})
		It("should convert the cluster_manager role", func() {
			var clusterObject = ClusterDescWithversion("1.3.0")
			var nodePool = opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "master",
			}))
		})
	})

	When("Constructing a STS for a NodePool with Keystore Values", func() {
		It("should create a proper initContainer", func() {
			mockSecretName := "some-secret"
			var clusterObject = ClusterDescWithKeystoreSecret(mockSecretName)
			var nodePool = opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(ContainElements([]corev1.VolumeMount{
				{
					Name:      "keystore",
					MountPath: "/tmp/keystore",
				},
				{
					Name:      "keystore-" + mockSecretName,
					MountPath: "/tmp/keystoreSecrets/" + mockSecretName,
				},
			}))
		})

		It("should mount the prefilled keystore into the opensearch container", func() {
			mockSecretName := "some-secret"
			var clusterObject = ClusterDescWithKeystoreSecret(mockSecretName)
			var nodePool = opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "keystore",
				MountPath: "/usr/share/opensearch/config/opensearch.keystore",
				SubPath:   "opensearch.keystore",
			}))
		})
	})
})
