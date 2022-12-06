package builders

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
)

func ClusterDescWithVersion(version string) opsterv1.OpenSearchCluster {
	return opsterv1.OpenSearchCluster{
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: version,
			},
		},
	}
}

func ClusterDescWithKeystoreSecret(secretName string, keyMappings map[string]string) opsterv1.OpenSearchCluster {
	return opsterv1.OpenSearchCluster{
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Keystore: []opsterv1.KeystoreValue{
					{
						Secret: corev1.LocalObjectReference{
							Name: secretName,
						},
						KeyMappings: keyMappings,
					},
				},
			},
		},
	}
}

func ClusterDescWithAdditionalConfigs(addtitionalConfig map[string]string, bootstrapAdditionalConfig map[string]string) opsterv1.OpenSearchCluster {
	return opsterv1.OpenSearchCluster{
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				AdditionalConfig: addtitionalConfig,
			},
			Bootstrap: opsterv1.BootstrapConfig{
				AdditionalConfig: bootstrapAdditionalConfig,
			},
		},
	}
}

var _ = Describe("Builders", func() {

	When("Constructing a STS for a NodePool", func() {
		It("should only use valid roles", func() {
			var clusterObject = ClusterDescWithVersion("2.2.1")
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
			var clusterObject = ClusterDescWithVersion("2.2.1")
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
			var clusterObject = ClusterDescWithVersion("1.3.0")
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
		It("should have annotations added to node", func() {
			var clusterObject = ClusterDescWithVersion("1.3.0")
			var nodePool = opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
				Annotations: map[string]string{
					"testAnnotationKey": "testAnnotationValue",
				},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Annotations).To(Equal(map[string]string{
				ConfigurationChecksumAnnotation: "foobar",
				"testAnnotationKey":             "testAnnotationValue",
			}))
		})
		It("should have a priority class name added to the node", func() {
			var clusterObject = ClusterDescWithVersion("1.3.0")
			var nodePool = opsterv1.NodePool{
				Component:         "masters",
				Roles:             []string{"cluster_manager"},
				PriorityClassName: "default",
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.PriorityClassName).To(Equal("default"))
		})
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			var clusterObject = ClusterDescWithVersion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			var result = NewSTSForNodePool("foobar", &clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:1.27.2-buildx"))
		})
		It("should use InitHelper.Image as InitHelper image if configured", func() {
			var clusterObject = ClusterDescWithVersion("2.2.1")
			customImage := "mycustomrepo.cr/custombusybox:1.2.3"
			clusterObject.Spec.InitHelper = opsterv1.InitHelperConfig{
				ImageSpec: &opsterv1.ImageSpec{
					Image: &customImage,
				},
			}
			var result = NewSTSForNodePool("foobar", &clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/custombusybox:1.2.3"))
		})
		It("should use defaults when no custom image is configured for InitHelper image", func() {
			var clusterObject = ClusterDescWithVersion("2.2.1")
			var result = NewSTSForNodePool("foobar", &clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("public.ecr.aws/opsterio/busybox:1.27.2-buildx"))
		})
		It("should use a custom dns name when env variable is set as cluster url", func() {
			customDns := "custom.domain"
			serviceName := "opensearch"
			namespace := "search"
			port := int32(9200)

			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.General.ServiceName = serviceName
			clusterObject.Namespace = namespace
			clusterObject.Spec.General.HttpPort = port

			os.Setenv(helpers.DnsBaseEnvVariable, customDns)

			actualUrl := URLForCluster(&clusterObject)
			expectedUrl := fmt.Sprintf("https://%s.%s.svc.%s:%d", serviceName, namespace, customDns, port)

			Expect(actualUrl).To(Equal(expectedUrl))
		})
	})

	When("Constructing a bootstrap pod", func() {
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			var clusterObject = ClusterDescWithVersion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			var result = NewBootstrapPod(&clusterObject, nil, nil)
			Expect(result.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:1.27.2-buildx"))
		})

		It("should apply the BootstrapNodeConfig to the env variables", func() {
			mockKey := "server.basePath"

			mockConfig := map[string]string{
				mockKey: "/opensearch-operated",
			}
			clusterObject := ClusterDescWithAdditionalConfigs(nil, mockConfig)
			result := NewBootstrapPod(&clusterObject, nil, nil)

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  mockKey,
				Value: mockConfig[mockKey],
			}))
		})

		It("should apply the General.AdditionalConfig to the env variables if not overwritten", func() {
			mockKey := "server.basePath"

			mockConfig := map[string]string{
				mockKey: "/opensearch-operated",
			}
			clusterObject := ClusterDescWithAdditionalConfigs(mockConfig, nil)
			result := NewBootstrapPod(&clusterObject, nil, nil)

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  mockKey,
				Value: mockConfig[mockKey],
			}))
		})

		It("should overwrite the General.AdditionalConfig with Bootstrap.AdditionalConfig when set", func() {
			mockKey1 := "server.basePath"
			mockKey2 := "server.rewriteBasePath"

			mockGeneralConfig := map[string]string{
				mockKey1: "/opensearch-operated",
			}
			mockBootstrapConfig := map[string]string{
				mockKey2: "false",
			}

			clusterObject := ClusterDescWithAdditionalConfigs(mockGeneralConfig, mockBootstrapConfig)
			result := NewBootstrapPod(&clusterObject, nil, nil)

			Expect(result.Spec.Containers[0].Env).NotTo(ContainElement(corev1.EnvVar{
				Name:  mockKey1,
				Value: mockGeneralConfig[mockKey2],
			}))

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  mockKey2,
				Value: mockBootstrapConfig[mockKey2],
			}))
		})
	})

	When("Constructing a STS for a NodePool with Keystore Values", func() {
		It("should create a proper initContainer", func() {
			mockSecretName := "some-secret"
			clusterObject := ClusterDescWithKeystoreSecret(mockSecretName, nil)
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}

			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
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
			clusterObject := ClusterDescWithKeystoreSecret(mockSecretName, nil)
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "keystore",
				MountPath: "/usr/share/opensearch/config/opensearch.keystore",
				SubPath:   "opensearch.keystore",
			}))
		})

		It("should properly rename secret keys when key mappings are given", func() {
			mockSecretName := "some-secret"
			oldKey := "old-key"
			newKey := "new-key"

			keyMappings := map[string]string{
				oldKey: newKey,
			}
			clusterObject := ClusterDescWithKeystoreSecret(mockSecretName, keyMappings)
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[1].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "keystore-" + mockSecretName,
				MountPath: "/tmp/keystoreSecrets/" + mockSecretName + "/" + newKey,
				SubPath:   oldKey,
			}))
		})
	})

	When("Checking for AllMastersReady", func() {
		It("should map all roles based on version", func() {
			namespaceName := "rolemapping"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			var clusterObject = ClusterDescWithVersion("2.2.1")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar"
			clusterObject.Spec.General.ServiceName = "foobar"
			var nodePool = opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			var sts = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})

		It("should handle a mapped master role", func() {
			namespaceName := "rolemapping-v1v2"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			var clusterObject = ClusterDescWithVersion("2.2.1")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar-v1v2"
			clusterObject.Spec.General.ServiceName = "foobar-v1v2"
			var nodePool = opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"master", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			var sts = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})

		It("should handle a v1 master role", func() {
			namespaceName := "rolemapping-v1"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			var clusterObject = ClusterDescWithVersion("1.3.0")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar-v1"
			clusterObject.Spec.General.ServiceName = "foobar-v1"
			var nodePool = opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"master", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			var sts = NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil, nil)
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})
	})
})
