package builders

import (
	"context"
	"fmt"
	"os"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
		It("should include the init containers as SKIP_INIT_CONTAINER is not set", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(len(result.Spec.Template.Spec.InitContainers)).To(Equal(1))
		})
		It("should skip the init container as SKIP_INIT_CONTAINER is set", func() {
			_ = os.Setenv(helpers.SkipInitContainerEnvVariable, "true")
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(len(result.Spec.Template.Spec.InitContainers)).To(Equal(0))
			_ = os.Unsetenv(helpers.SkipInitContainerEnvVariable)
		})
		It("should include the init containers as SKIP_INIT_CONTAINER is not set", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})
			Expect(len(result.Spec.InitContainers)).To(Equal(1))
		})
		It("should skip the init container as SKIP_INIT_CONTAINER is set", func() {
			_ = os.Setenv(helpers.SkipInitContainerEnvVariable, "true")
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})
			Expect(len(result.Spec.InitContainers)).To(Equal(0))
			_ = os.Unsetenv(helpers.SkipInitContainerEnvVariable)
		})
		It("should only use valid roles", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "cluster_manager,ingest",
			}))
		})
		It("should convert the master role", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"master"},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "cluster_manager",
			}))
		})
		It("should convert the cluster_manager role", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "master",
			}))
		})
		It("should have annotations added to node", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
				Annotations: map[string]string{
					"testAnnotationKey": "testAnnotationValue",
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Annotations).To(Equal(map[string]string{
				ConfigurationChecksumAnnotation: "foobar",
				"testAnnotationKey":             "testAnnotationValue",
			}))
		})
		It("should have annotations added to sts", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
				Annotations: map[string]string{
					"testAnnotationKey": "testAnnotationValue",
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Annotations).To(Equal(map[string]string{
				ConfigurationChecksumAnnotation: "foobar",
				"testAnnotationKey":             "testAnnotationValue",
			}))
		})
		It("should have a priority class name added to the node", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opsterv1.NodePool{
				Component:         "masters",
				Roles:             []string{"cluster_manager"},
				PriorityClassName: "default",
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.PriorityClassName).To(Equal("default"))
		})
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:latest"))
		})
		It("should use InitHelper.Image as InitHelper image if configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			customImage := "mycustomrepo.cr/custombusybox:1.2.3"
			clusterObject.Spec.InitHelper = opsterv1.InitHelperConfig{
				ImageSpec: &opsterv1.ImageSpec{
					Image: &customImage,
				},
			}
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/custombusybox:1.2.3"))
		})
		It("should use defaults when no custom image is configured for InitHelper image", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("docker.io/busybox:latest"))
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

		It("should properly setup the main command when installing plugins", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			pluginA := "some-plugin"
			pluginB := "another-plugin"

			clusterObject.Spec.General.PluginsList = []string{pluginA, pluginB}
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})

			installCmd := fmt.Sprintf(
				"./bin/opensearch-plugin install --batch '%s' '%s' && ./opensearch-docker-entrypoint.sh",
				pluginA,
				pluginB,
			)

			expected := []string{
				"/bin/bash",
				"-c",
				installCmd,
			}

			actual := result.Spec.Template.Spec.Containers[0].Command

			Expect(expected).To(Equal(actual))
		})

		It("should add experimental flag when the node.roles contains search and the version is below 2.7", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, CommonEnvVars(&clusterObject, ""))
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "search",
			}))

			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx512M -Xms512M -Dopensearch.experimental.feature.searchable_snapshot.enabled=true -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should not add experimental flag when the node.roles contains search and the version is 2.7 or above", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, CommonEnvVars(&clusterObject, ""))
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "search",
			}))

			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx512M -Xms512M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should properly configure security contexts if set", func() {
			user := int64(1000)
			podSecurityContext := &corev1.PodSecurityContext{
				RunAsUser:    &user,
				RunAsGroup:   &user,
				RunAsNonRoot: pointer.Bool(true),
			}
			securityContext := &corev1.SecurityContext{
				Privileged:               pointer.Bool(false),
				AllowPrivilegeEscalation: pointer.Bool(false),
			}
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.General.PodSecurityContext = podSecurityContext
			clusterObject.Spec.General.SecurityContext = securityContext
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.SecurityContext).To(Equal(podSecurityContext))
			Expect(result.Spec.Template.Spec.Containers[0].SecurityContext).To(Equal(securityContext))
		})
		It("should use default storageclass if not specified", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				Persistence: &opsterv1.PersistenceConfig{PersistenceSource: opsterv1.PersistenceSource{
					PVC: &opsterv1.PVCSource{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			var expected *string = nil
			actual := result.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
			Expect(expected).To(Equal(actual))
		})
		It("should set jvm to half of memory request when memory request is set and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx1024M -Xms1024M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set jvm to half of memory request when memory request is fraction and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1.5Gi"),
					},
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx768M -Xms768M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should set jvm to half of memory request when memory request is set in G and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2G"),
					},
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx953M -Xms953M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set jvm to default when memory request and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx512M -Xms512M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set NodePool.Jvm as jvm when it jvm is provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Jvm: "-Xmx1024M -Xms1024M",
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx1024M -Xms1024M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set NodePool.jvm as jvm when jvm and memory request are provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opsterv1.NodePool{
				Jvm: "-Xmx1024M -Xms1024M",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xmx1024M -Xms1024M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
	})

	When("Constructing a bootstrap pod", func() {
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			result := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:latest"))
		})

		It("should apply the BootstrapNodeConfig to the env variables", func() {
			mockKey := "server.basePath"

			mockConfig := map[string]string{
				mockKey: "/opensearch-operated",
			}
			clusterObject := ClusterDescWithAdditionalConfigs(nil, mockConfig)
			result := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  mockKey,
				Value: mockConfig[mockKey],
			}))
		})

		It("should include OPENSEARCH_INITIAL_ADMIN_PASSWORD env var pointing to the supplied secret name", func() {
			mockKey := "server.basePath"
			mockSecretName := "fake-secret"

			mockConfig := map[string]string{
				mockKey: "/opensearch-operated",
			}
			clusterObject := ClusterDescWithAdditionalConfigs(mockConfig, nil)
			result := NewBootstrapPod(&clusterObject, nil, nil, CommonEnvVars(&clusterObject, mockSecretName))

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name: "OPENSEARCH_INITIAL_ADMIN_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: mockSecretName},
						Key:                  "password",
						Optional:             pointer.Bool(false),
					},
				},
			}))
		})

		It("should apply the General.AdditionalConfig to the env variables if not overwritten", func() {
			mockKey := "server.basePath"

			mockConfig := map[string]string{
				mockKey: "/opensearch-operated",
			}
			clusterObject := ClusterDescWithAdditionalConfigs(mockConfig, nil)
			result := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})

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
			result := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})

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

			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
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
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
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
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
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
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar"
			clusterObject.Spec.General.ServiceName = "foobar"
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})

		It("should handle a mapped master role", func() {
			namespaceName := "rolemapping-v1v2"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar-v1v2"
			clusterObject.Spec.General.ServiceName = "foobar-v1v2"
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"master", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})

		It("should handle a v1 master role", func() {
			namespaceName := "rolemapping-v1"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			clusterObject := ClusterDescWithVersion("1.3.0")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar-v1"
			clusterObject.Spec.General.ServiceName = "foobar-v1"
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"master", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})
	})

	When("Using custom command for OpenSearch startup", func() {
		It("it should use the specified startup command", func() {
			namespaceName := "customcommand"
			customCommand := "/myentrypoint.sh"
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.ObjectMeta.Namespace = namespaceName
			clusterObject.ObjectMeta.Name = "foobar"
			clusterObject.Spec.General.Command = customCommand
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(sts.Spec.Template.Spec.Containers[0].Command[2]).To(Equal(customCommand))
		})
	})

	When("Configuring a serviceAccount", func() {
		It("should set it for all cluster pods and the securityconfig-update job", func() {
			const serviceAccount = "my-test-serviceaccount"
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.ObjectMeta.Namespace = "foobar"
			clusterObject.ObjectMeta.Name = "foobar"
			clusterObject.Spec.General.ServiceAccount = serviceAccount
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(sts.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))

			job := NewSecurityconfigUpdateJob(&clusterObject, "foobar", "foobar", "foobar", "admin-cert", "cmd", nil, nil)
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
		})
	})

	When("building services with annotations", func() {
		It("should populate the NewServiceForCR function with ", func() {
			clusterName := "opensearch"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName: clusterName,
						Annotations: map[string]string{
							"testAnnotationKey":  "testValue",
							"testAnnotationKey2": "testValue2",
						},
					},
				},
			}
			result := NewServiceForCR(&spec)
			Expect(result.Annotations).To(Equal(map[string]string{
				"testAnnotationKey":  "testValue",
				"testAnnotationKey2": "testValue2",
			}))
		})

		It("should populate the NewHeadlessServiceForNodePool function with ", func() {
			clusterName := "opensearch"
			nodePool := opsterv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				Annotations: map[string]string{
					"testAnnotationKey": "testValue",
				},
			}
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName: clusterName,
						Annotations: map[string]string{
							"testAnnotationKey2": "testValue2",
						},
					},
				},
			}
			result := NewHeadlessServiceForNodePool(&spec, &nodePool)
			Expect(result.Annotations).To(Equal(map[string]string{
				"testAnnotationKey":  "testValue",
				"testAnnotationKey2": "testValue2",
			}))
		})
	})

	When("Using custom probe timeouts and thresholds for OpenSearch startup", func() {
		It("should have default probes timeouts and thresholds", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold).To(Equal(int32(10)))

			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.FailureThreshold).To(Equal(int32(10)))

			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds).To(Equal(int32(60)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(Equal(int32(5)))
		})

		It("should have use probes timeouts and thresholds as in given config only for single value change", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
				Probes: &opsterv1.ProbesConfig{
					Liveness: &opsterv1.ProbeConfig{
						FailureThreshold: 15,
					},
					Startup: &opsterv1.ProbeConfig{
						FailureThreshold: 11,
					},
					Readiness: &opsterv1.ReadinessProbeConfig{
						FailureThreshold: 9,
					},
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold).To(Equal(int32(15)))

			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.FailureThreshold).To(Equal(int32(11)))

			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds).To(Equal(int32(60)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(Equal(int32(9)))
		})

		It("should have use probes timeouts and thresholds as in given config only for all values changed", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opsterv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
				Probes: &opsterv1.ProbesConfig{
					Liveness: &opsterv1.ProbeConfig{
						InitialDelaySeconds: 12,
						TimeoutSeconds:      6,
						PeriodSeconds:       25,
						SuccessThreshold:    2,
						FailureThreshold:    15,
					},
					Startup: &opsterv1.ProbeConfig{
						InitialDelaySeconds: 14,
						TimeoutSeconds:      7,
						PeriodSeconds:       27,
						SuccessThreshold:    3,
						FailureThreshold:    11,
					},
					Readiness: &opsterv1.ReadinessProbeConfig{
						InitialDelaySeconds: 65,
						TimeoutSeconds:      34,
						PeriodSeconds:       33,
						FailureThreshold:    9,
					},
				},
			}
			result := NewSTSForNodePool(&clusterObject, nodePool, "foobar", nil, nil, nil, []corev1.EnvVar{})
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).To(Equal(int32(12)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).To(Equal(int32(6)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds).To(Equal(int32(25)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold).To(Equal(int32(2)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold).To(Equal(int32(15)))

			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.InitialDelaySeconds).To(Equal(int32(14)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.TimeoutSeconds).To(Equal(int32(7)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.PeriodSeconds).To(Equal(int32(27)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.SuccessThreshold).To(Equal(int32(3)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.FailureThreshold).To(Equal(int32(11)))

			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds).To(Equal(int32(65)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds).To(Equal(int32(34)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds).To(Equal(int32(33)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(Equal(int32(9)))
		})
	})

	When("Configuring InitHelper Resources", func() {
		It("should propagate Resources to all init containers", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.InitHelper = opsterv1.InitHelperConfig{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			}
			nodePoolSts := NewSTSForNodePool(&clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil, []corev1.EnvVar{})
			for _, container := range nodePoolSts.Spec.Template.Spec.InitContainers {
				Expect(container.Resources).To(Equal(clusterObject.Spec.InitHelper.Resources))
			}
			bootstrapPod := NewBootstrapPod(&clusterObject, nil, nil, []corev1.EnvVar{})
			for _, container := range bootstrapPod.Spec.InitContainers {
				Expect(container.Resources).To(Equal(clusterObject.Spec.InitHelper.Resources))
			}
		})
	})

	When("Configuring Security Config UpdateJob Resources", func() {
		It("should propagate Resources to the Security Config UpdateJob", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.Security = &opsterv1.Security{
				Config: &opsterv1.SecurityConfig{
					UpdateJob: opsterv1.SecurityUpdateJobConfig{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				},
			}

			job := NewSecurityconfigUpdateJob(&clusterObject, "dummy", "dummy", "dummy", "dummy", "dummy", nil, nil)
			Expect(job.Spec.Template.Spec.Containers[0].Resources).To(Equal(clusterObject.Spec.Security.Config.UpdateJob.Resources))
		})

		It("should propagate Resources to the Security Config UpdateJob if partially configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.Security = &opsterv1.Security{
				Config: &opsterv1.SecurityConfig{
					UpdateJob: opsterv1.SecurityUpdateJobConfig{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("1"),
							},
						},
					},
				},
			}

			job := NewSecurityconfigUpdateJob(&clusterObject, "dummy", "dummy", "dummy", "dummy", "dummy", nil, nil)
			Expect(job.Spec.Template.Spec.Containers[0].Resources).To(Equal(clusterObject.Spec.Security.Config.UpdateJob.Resources))
		})
	})
})
