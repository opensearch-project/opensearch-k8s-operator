package builders

import (
	"context"
	"fmt"
	"os"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ClusterDescWithVersion(version string) opensearchv1.OpenSearchCluster {
	return opensearchv1.OpenSearchCluster{
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{
				Version: version,
			},
		},
	}
}

func ClusterDescWithKeystoreSecret(secretName string, keyMappings map[string]string) opensearchv1.OpenSearchCluster {
	return opensearchv1.OpenSearchCluster{
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{
				Keystore: []opensearchv1.KeystoreValue{
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

func ClusterDescWithBootstrapKeystoreSecret(secretName string, keyMappings map[string]string) opensearchv1.OpenSearchCluster {
	return opensearchv1.OpenSearchCluster{
		Spec: opensearchv1.ClusterSpec{
			Bootstrap: opensearchv1.BootstrapConfig{
				Keystore: []opensearchv1.KeystoreValue{
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

func ClusterDescWithAdditionalConfigs(addtitionalConfig map[string]string, bootstrapEnv []corev1.EnvVar) opensearchv1.OpenSearchCluster {
	return opensearchv1.OpenSearchCluster{
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{
				AdditionalConfig: addtitionalConfig,
			},
			Bootstrap: opensearchv1.BootstrapConfig{
				Env: bootstrapEnv,
			},
		},
	}
}

var _ = Describe("Builders", func() {
	When("Constructing a STS for a NodePool", func() {
		It("should include the init containers as SKIP_INIT_CONTAINER is not set", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
			Expect(len(result.Spec.Template.Spec.InitContainers)).To(Equal(1))
		})
		It("should skip the init container as SKIP_INIT_CONTAINER is set", func() {
			_ = os.Setenv(helpers.SkipInitContainerEnvVariable, "true")
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
			Expect(len(result.Spec.Template.Spec.InitContainers)).To(Equal(0))
			_ = os.Unsetenv(helpers.SkipInitContainerEnvVariable)
		})
		It("should include the init containers as SKIP_INIT_CONTAINER is not set", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(len(result.Spec.InitContainers)).To(Equal(1))
		})
		It("should skip the init container as SKIP_INIT_CONTAINER is set", func() {
			_ = os.Setenv(helpers.SkipInitContainerEnvVariable, "true")
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(len(result.Spec.InitContainers)).To(Equal(0))
			_ = os.Unsetenv(helpers.SkipInitContainerEnvVariable)
		})
		It("should only use valid roles", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "cluster_manager,ingest",
			}))
		})
		It("should convert the master role", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"master"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "cluster_manager",
			}))
		})
		It("should convert the cluster_manager role", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "master",
			}))
		})
		It("should accept the warm role", func() {
			clusterObject := ClusterDescWithVersion("3.0.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"warm"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "warm",
			}))
		})
		It("should convert the warm role", func() {
			clusterObject := ClusterDescWithVersion("2.0.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"warm"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "search",
			}))
		})
		It("should set node.roles to [] for coordinator-only nodes (OpenSearch 3.0+)", func() {
			clusterObject := ClusterDescWithVersion("3.0.0")
			nodePool := opensearchv1.NodePool{
				Component: "coordinators",
				Roles:     []string{},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "[]",
			}))
		})
		It("should have annotations added to node", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
				Annotations: map[string]string{
					"testAnnotationKey": "testAnnotationValue",
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Annotations).To(Equal(map[string]string{
				ConfigurationChecksumAnnotation: "foobar",
				"testAnnotationKey":             "testAnnotationValue",
			}))
		})
		It("should have annotations added to sts", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
				Annotations: map[string]string{
					"testAnnotationKey": "testAnnotationValue",
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Annotations).To(Equal(map[string]string{
				ConfigurationChecksumAnnotation: "foobar",
				"testAnnotationKey":             "testAnnotationValue",
			}))
		})
		It("should have a priority class name added to the node", func() {
			clusterObject := ClusterDescWithVersion("1.3.0")
			nodePool := opensearchv1.NodePool{
				Component:         "masters",
				Roles:             []string{"cluster_manager"},
				PriorityClassName: "default",
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.PriorityClassName).To(Equal("default"))
		})
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:latest"))
		})
		It("should use InitHelper.Image as InitHelper image if configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			customImage := "mycustomrepo.cr/custombusybox:1.2.3"
			clusterObject.Spec.InitHelper = opensearchv1.InitHelperConfig{
				ImageSpec: &opensearchv1.ImageSpec{
					Image: &customImage,
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/custombusybox:1.2.3"))
		})
		It("should use defaults when no custom image is configured for InitHelper image", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
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

			_ = os.Setenv(helpers.DnsBaseEnvVariable, customDns)

			actualUrl := URLForCluster(&clusterObject)
			expectedUrl := fmt.Sprintf("http://%s.%s.svc.%s:%d", serviceName, namespace, customDns, port)

			Expect(actualUrl).To(Equal(expectedUrl))
		})

		It("should use operatorClusterURL when provided", func() {
			customHost := "opensearch.example.com"
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.General.OperatorClusterURL = &customHost

			actualUrl := URLForCluster(&clusterObject)
			// When HttpPort is 0 (default), ClusterURL should default to 9200
			expectedUrl := fmt.Sprintf("http://%s:9200", customHost)
			Expect(actualUrl).To(Equal(expectedUrl))
		})

		It("should properly setup the main command when installing plugins", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			pluginA := "some-plugin"
			pluginB := "another-plugin"

			clusterObject.Spec.General.PluginsList = []string{pluginA, pluginB}
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)

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
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "search",
			}))

			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms512M -Xmx512M -Dopensearch.experimental.feature.searchable_snapshot.enabled=true -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should not add experimental flag when the node.roles contains search and the version is 2.7 or above", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "node.roles",
				Value: "search",
			}))

			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms512M -Xmx512M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should properly configure security contexts if set", func() {
			user := int64(1000)
			podSecurityContext := &corev1.PodSecurityContext{
				RunAsUser:    &user,
				RunAsGroup:   &user,
				RunAsNonRoot: ptr.To(true),
			}
			securityContext := &corev1.SecurityContext{
				Privileged:               ptr.To(false),
				AllowPrivilegeEscalation: ptr.To(false),
			}
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.General.PodSecurityContext = podSecurityContext
			clusterObject.Spec.General.SecurityContext = securityContext
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.SecurityContext).To(Equal(podSecurityContext))
			Expect(result.Spec.Template.Spec.Containers[0].SecurityContext).To(Equal(securityContext))
		})
		It("should use default storageclass if no persistence specified", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				// No persistence specified
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			var expected *string = nil
			actual := result.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
			Expect(expected).To(Equal(actual))
		})
		It("should use default storageClass when persistence is specified without storageClass", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				Persistence: &opensearchv1.PersistenceConfig{PersistenceSource: opensearchv1.PersistenceSource{
					PVC: &opensearchv1.PVCSource{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			var expected *string = nil
			actual := result.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
			Expect(expected).To(Equal(actual))
		})
		It("should create empty storageClassName when explicitly set to empty", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			emptyString := ""
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				Persistence: &opensearchv1.PersistenceConfig{PersistenceSource: opensearchv1.PersistenceSource{
					PVC: &opensearchv1.PVCSource{
						StorageClassName: &emptyString,
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			expected := &emptyString
			actual := result.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
			Expect(expected).To(Equal(actual))
		})
		It("should use specific storageClassName when provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			specificClass := "fast-ssd"
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				Persistence: &opensearchv1.PersistenceConfig{PersistenceSource: opensearchv1.PersistenceSource{
					PVC: &opensearchv1.PVCSource{
						StorageClassName: &specificClass,
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			expected := &specificClass
			actual := result.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
			Expect(expected).To(Equal(actual))
		})
		It("should set jvm to half of memory request when memory request is set and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms1024M -Xmx1024M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set jvm to half of memory request when memory request is fraction and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1.5Gi"),
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms768M -Xmx768M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should set jvm to half of memory request when memory request is set in G and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2G"),
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms953M -Xmx953M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set jvm to default when memory request and jvm are not provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms512M -Xmx512M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set NodePool.Jvm as jvm when it jvm is provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Jvm: "-Xms1024M -Xmx1024M",
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms1024M -Xmx1024M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})
		It("should set NodePool.jvm as jvm when jvm and memory request are provided", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Jvm: "-Xms1024M -Xmx1024M",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "OPENSEARCH_JAVA_OPTS",
				Value: "-Xms1024M -Xmx1024M -Dopensearch.transport.cname_in_publish_address=true",
			}))
		})

		It("should include sidecar containers when specified", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				SidecarContainers: []corev1.Container{
					{
						Name:  "log-shipper",
						Image: "fluent/fluent-bit:latest",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("64Mi"),
								corev1.ResourceCPU:    resource.MustParse("100m"),
							},
						},
					},
					{
						Name:  "metrics-collector",
						Image: "prom/node-exporter:latest",
						Ports: []corev1.ContainerPort{
							{
								Name:          "metrics",
								ContainerPort: 9100,
								Protocol:      "TCP",
							},
						},
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)

			// Should have 3 containers total: 1 main OpenSearch + 2 additional
			Expect(len(result.Spec.Template.Spec.Containers)).To(Equal(3))

			// First container should be the main OpenSearch container
			Expect(result.Spec.Template.Spec.Containers[0].Name).To(Equal("opensearch"))

			// Second container should be the first additional container
			Expect(result.Spec.Template.Spec.Containers[1].Name).To(Equal("log-shipper"))
			Expect(result.Spec.Template.Spec.Containers[1].Image).To(Equal("fluent/fluent-bit:latest"))
			Expect(result.Spec.Template.Spec.Containers[1].Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("64Mi")))
			Expect(result.Spec.Template.Spec.Containers[1].Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("100m")))

			// Third container should be the second additional container
			Expect(result.Spec.Template.Spec.Containers[2].Name).To(Equal("metrics-collector"))
			Expect(result.Spec.Template.Spec.Containers[2].Image).To(Equal("prom/node-exporter:latest"))
			Expect(len(result.Spec.Template.Spec.Containers[2].Ports)).To(Equal(1))
			Expect(result.Spec.Template.Spec.Containers[2].Ports[0].Name).To(Equal("metrics"))
			Expect(result.Spec.Template.Spec.Containers[2].Ports[0].ContainerPort).To(Equal(int32(9100)))
		})
		It("should include custom init containers that run before main container", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			initContainer := corev1.Container{
				Name:  "custom-init",
				Image: "custom-init:latest",
			}
			nodePool := opensearchv1.NodePool{
				InitContainers: []corev1.Container{initContainer},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers).To(ContainElement(corev1.Container{
				Name:  "custom-init",
				Image: "custom-init:latest",
			}))
			for _, container := range result.Spec.Template.Spec.Containers {
				Expect(container.Name).NotTo(Equal("custom-init"))
			}
		})

		It("should include multiple custom init containers when specified", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			initContainer1 := corev1.Container{
				Name:  "custom-init1",
				Image: "custom-init1:latest",
			}
			initContainer2 := corev1.Container{
				Name:  "custom-init2",
				Image: "custom-init2:latest",
			}
			result := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{
				Roles:          []string{"cluster_manager"},
				InitContainers: []corev1.Container{initContainer1, initContainer2},
			}, "foobar", nil, nil)
			Expect(len(result.Spec.Template.Spec.InitContainers)).To(Equal(3))
			Expect(result.Spec.Template.Spec.InitContainers[0].Name).To(Equal("custom-init1"))
			Expect(result.Spec.Template.Spec.InitContainers[1].Name).To(Equal("custom-init2"))
		})
	})

	When("Constructing a bootstrap pod", func() {
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			result := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(result.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:latest"))
		})

		It("should apply the ENV to the env variables", func() {
			mockKey := "server.basePath"

			mockEnv := []corev1.EnvVar{
				{
					Name:  mockKey,
					Value: "/opensearch-operated",
				},
			}
			clusterObject := ClusterDescWithAdditionalConfigs(nil, mockEnv)
			result := NewBootstrapPod(&clusterObject, nil, nil)

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  mockKey,
				Value: "/opensearch-operated",
			}))
		})

		It("should apply bootstrap pod annotations", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			expectedAnnotations := map[string]string{
				"custom-annotation":  "custom-value",
				"another-annotation": "another-value",
			}
			clusterObject.Spec.Bootstrap.Annotations = expectedAnnotations

			result := NewBootstrapPod(&clusterObject, nil, nil)

			Expect(result.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
		})

		It("should apply Bootstrap.Env when set", func() {
			mockKey1 := "server.basePath"
			mockKey2 := "server.rewriteBasePath"

			mockGeneralConfig := map[string]string{
				mockKey1: "/opensearch-operated",
			}
			mockBootstrapEnv := []corev1.EnvVar{
				{
					Name:  mockKey2,
					Value: "false",
				},
			}

			clusterObject := ClusterDescWithAdditionalConfigs(mockGeneralConfig, mockBootstrapEnv)
			result := NewBootstrapPod(&clusterObject, nil, nil)

			Expect(result.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  mockKey2,
				Value: "false",
			}))
		})
		It("should properly setup the main command when installing plugins", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			pluginA := "some-plugin"
			pluginB := "another-plugin"

			clusterObject.Spec.Bootstrap.PluginsList = []string{pluginA, pluginB}
			result := NewBootstrapPod(&clusterObject, nil, nil)

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

			actual := result.Spec.Containers[0].Command

			Expect(expected).To(Equal(actual))
		})

		It("should inherit General.PluginsList when Bootstrap.PluginsList is not set", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			pluginA := "repository-s3"
			pluginB := "analysis-icu"

			clusterObject.Spec.General.PluginsList = []string{pluginA, pluginB}
			// Bootstrap.PluginsList is not set
			result := NewBootstrapPod(&clusterObject, nil, nil)

			actual := result.Spec.Containers[0].Command
			Expect(len(actual)).To(Equal(3))
			Expect(actual[2]).To(ContainSubstring(pluginA))
			Expect(actual[2]).To(ContainSubstring(pluginB))
		})

		It("should override General.PluginsList with Bootstrap.PluginsList when explicitly set", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			generalPlugin := "repository-s3"
			bootstrapPluginA := "custom-plugin-a"
			bootstrapPluginB := "custom-plugin-b"

			clusterObject.Spec.General.PluginsList = []string{generalPlugin}
			clusterObject.Spec.Bootstrap.PluginsList = []string{bootstrapPluginA, bootstrapPluginB}
			result := NewBootstrapPod(&clusterObject, nil, nil)

			// Should use Bootstrap.PluginsList, not General.PluginsList
			actual := result.Spec.Containers[0].Command
			Expect(len(actual)).To(Equal(3))
			Expect(actual[2]).To(ContainSubstring(bootstrapPluginA))
			Expect(actual[2]).To(ContainSubstring(bootstrapPluginB))
			Expect(actual[2]).NotTo(ContainSubstring(generalPlugin))
		})

		It("should use no plugins when both General.PluginsList and Bootstrap.PluginsList are empty", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			// Neither list is set
			result := NewBootstrapPod(&clusterObject, nil, nil)

			actual := result.Spec.Containers[0].Command
			Expect(len(actual)).To(Equal(3))
			Expect(actual[2]).To(Equal("./opensearch-docker-entrypoint.sh"))
		})

		It("should use PVC for data volume instead of emptyDir", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPod(&clusterObject, nil, nil)

			// Find the data volume
			var dataVolume *corev1.Volume
			for i, volume := range result.Spec.Volumes {
				if volume.Name == "data" {
					dataVolume = &result.Spec.Volumes[i]
					break
				}
			}

			Expect(dataVolume).NotTo(BeNil())
			Expect(dataVolume.VolumeSource.PersistentVolumeClaim).NotTo(BeNil())
			Expect(dataVolume.VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal(fmt.Sprintf("%s-bootstrap-data", clusterObject.Name)))
			Expect(dataVolume.VolumeSource.EmptyDir).To(BeNil())
		})
	})

	When("Constructing a bootstrap pod with Keystore Values", func() {
		It("should create a proper initContainer", func() {
			mockSecretName := "some-secret"
			clusterObject := ClusterDescWithBootstrapKeystoreSecret(mockSecretName, nil)

			result := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(result.Spec.InitContainers[1].VolumeMounts).To(ContainElements([]corev1.VolumeMount{
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
			clusterObject := ClusterDescWithBootstrapKeystoreSecret(mockSecretName, nil)
			result := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(result.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
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
			clusterObject := ClusterDescWithBootstrapKeystoreSecret(mockSecretName, keyMappings)
			result := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(result.Spec.InitContainers[1].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "keystore-" + mockSecretName,
				MountPath: "/tmp/keystoreSecrets/" + mockSecretName + "/" + newKey,
				SubPath:   oldKey,
			}))
		})
		When("Constructing a bootstrap pod with Volumes", func() {
			It("should include all the required volumes and mounts", func() {
				clusterObject := opensearchv1.OpenSearchCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-namespace",
					},
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{
							PluginsList: []string{"repository-s3"},
						},
					},
				}

				// Create the volumes that would come from the configuration reconciler
				volumes := []corev1.Volume{
					{
						Name: "rw-conf",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "rw-logs",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "rw-plugins",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}

				volumeMounts := []corev1.VolumeMount{
					{
						Name:      "rw-conf",
						MountPath: "/usr/share/opensearch/conf",
					},
					{
						Name:      "rw-logs",
						MountPath: "/usr/share/opensearch/logs",
					},
					{
						Name:      "rw-plugins",
						MountPath: "/usr/share/opensearch/plugins",
					},
				}

				result := NewBootstrapPod(&clusterObject, volumes, volumeMounts)

				Expect(len(result.Spec.Volumes)).To(Equal(4))
				Expect(result.Spec.Volumes[0].Name).To(Equal(volumes[0].Name))
				Expect(result.Spec.Volumes[1].Name).To(Equal(volumes[1].Name))
				Expect(result.Spec.Volumes[2].Name).To(Equal(volumes[2].Name))

				Expect(len(result.Spec.Containers)).To(Equal(1))
				Expect(len(result.Spec.Containers[0].VolumeMounts)).To(Equal(4))
				Expect(result.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(volumeMounts[0].Name))
				Expect(result.Spec.Containers[0].VolumeMounts[1].Name).To(Equal(volumeMounts[1].Name))
				Expect(result.Spec.Containers[0].VolumeMounts[2].Name).To(Equal(volumeMounts[2].Name))
			})
		})
	})

	When("Constructing a bootstrap PVC", func() {
		It("should create a PVC with correct name and storage size", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPVC(&clusterObject)

			expectedName := fmt.Sprintf("%s-bootstrap-data", clusterObject.Name)
			Expect(result.Name).To(Equal(expectedName))
			Expect(result.Namespace).To(Equal(clusterObject.Namespace))
			Expect(result.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
			Expect(result.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))
		})

		It("should use custom storage size from bootstrap resources", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.Bootstrap.DiskSize = resource.MustParse("2Gi")
			result := NewBootstrapPVC(&clusterObject)

			Expect(result.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("2Gi")))
		})

		It("should have correct labels for cluster identification", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			result := NewBootstrapPVC(&clusterObject)

			Expect(result.Labels).To(HaveKeyWithValue(helpers.ClusterLabel, clusterObject.Name))
		})
	})

	When("Constructing a STS for a NodePool with Keystore Values", func() {
		It("should create a proper initContainer", func() {
			mockSecretName := "some-secret"
			clusterObject := ClusterDescWithKeystoreSecret(mockSecretName, nil)
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}

			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
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
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
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
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager", "foobar", "ingest"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
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
			clusterObject.Namespace = namespaceName
			clusterObject.Name = "foobar"
			clusterObject.Spec.General.ServiceName = "foobar"
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})

		It("should handle a mapped master role", func() {
			namespaceName := "rolemapping-v1v2"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Namespace = namespaceName
			clusterObject.Name = "foobar-v1v2"
			clusterObject.Spec.General.ServiceName = "foobar-v1v2"
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"master", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			sts.Status.ReadyReplicas = 2
			Expect(k8sClient.Create(context.Background(), sts)).To(Not(HaveOccurred()))
			result := AllMastersReady(context.Background(), k8sClient, &clusterObject)
			Expect(result).To(BeFalse())
		})

		It("should handle a v1 master role", func() {
			namespaceName := "rolemapping-v1"
			Expect(CreateNamespace(k8sClient, namespaceName)).Should(Succeed())
			clusterObject := ClusterDescWithVersion("1.3.0")
			clusterObject.Namespace = namespaceName
			clusterObject.Name = "foobar-v1"
			clusterObject.Spec.General.ServiceName = "foobar-v1"
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"master", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
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
			clusterObject.Namespace = namespaceName
			clusterObject.Name = "foobar"
			clusterObject.Spec.General.Command = customCommand
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(sts.Spec.Template.Spec.Containers[0].Command[2]).To(Equal(customCommand))
		})
	})

	When("Configuring a serviceAccount", func() {
		It("should set it for all cluster pods and the securityconfig-update job", func() {
			const serviceAccount = "my-test-serviceaccount"
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Namespace = "foobar"
			clusterObject.Name = "foobar"
			clusterObject.Spec.General.ServiceAccount = serviceAccount
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(sts.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))

			job := NewSecurityconfigUpdateJob(&clusterObject, "foobar", "foobar", "foobar", "admin-cert", "cmd", nil, nil)
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
		})
	})

	When("building services with annotations", func() {
		It("should populate the NewServiceForCR function with ", func() {
			clusterName := "opensearch"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
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
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
				Annotations: map[string]string{
					"testAnnotationKey": "testValue",
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
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
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold).To(Equal(int32(10)))

			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.TimeoutSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.FailureThreshold).To(Equal(int32(10)))

			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds).To(Equal(int32(60)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(Equal(int32(5)))
		})

		It("should have use probes timeouts and thresholds as in given config only for single value change", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
				Probes: &opensearchv1.ProbesConfig{
					Liveness: &opensearchv1.ProbeConfig{
						FailureThreshold: 15,
					},
					Startup: &opensearchv1.CommandProbeConfig{
						FailureThreshold: 11,
					},
					Readiness: &opensearchv1.CommandProbeConfig{
						FailureThreshold: 9,
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold).To(Equal(int32(15)))

			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.TimeoutSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.FailureThreshold).To(Equal(int32(11)))

			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds).To(Equal(int32(60)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds).To(Equal(int32(30)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(Equal(int32(9)))
		})

		It("should have use probes timeouts and thresholds as in given config only for all values changed", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
				Probes: &opensearchv1.ProbesConfig{
					Liveness: &opensearchv1.ProbeConfig{
						InitialDelaySeconds: 12,
						TimeoutSeconds:      6,
						PeriodSeconds:       25,
						SuccessThreshold:    2,
						FailureThreshold:    15,
					},
					Startup: &opensearchv1.CommandProbeConfig{
						InitialDelaySeconds: 14,
						TimeoutSeconds:      7,
						PeriodSeconds:       27,
						SuccessThreshold:    3,
						FailureThreshold:    11,
					},
					Readiness: &opensearchv1.CommandProbeConfig{
						InitialDelaySeconds: 65,
						TimeoutSeconds:      34,
						PeriodSeconds:       33,
						SuccessThreshold:    4,
						FailureThreshold:    9,
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
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
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.SuccessThreshold).To(Equal(int32(4)))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold).To(Equal(int32(9)))
		})
	})

	When("Using custom command for OpenSearch probes", func() {
		It("should have default command when not set", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.ProbeHandler.Exec.Command).
				To(Equal([]string{"/bin/bash", "-c", "curl -k -u \"$(cat /mnt/admin-credentials/username):$(cat /mnt/admin-credentials/password)\" --silent --fail 'http://localhost:9200'"}))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command).
				To(Equal([]string{"/bin/bash", "-c", "curl -k -u \"$(cat /mnt/admin-credentials/username):$(cat /mnt/admin-credentials/password)\" --silent --fail 'http://localhost:9200'"}))
		})

		It("should have custom command when set", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"search"},
				Probes: &opensearchv1.ProbesConfig{
					Startup: &opensearchv1.CommandProbeConfig{
						Command: []string{"/bin/bash", "-c", "echo 'startup'"},
					},
					Readiness: &opensearchv1.CommandProbeConfig{
						Command: []string{"/bin/bash", "-c", "echo 'ready'"},
					},
				},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.ProbeHandler.Exec.Command).
				To(Equal([]string{"/bin/bash", "-c", "echo 'startup'"}))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command).
				To(Equal([]string{"/bin/bash", "-c", "echo 'ready'"}))
		})
	})

	When("HTTP TLS is disabled", func() {
		It("should use http protocol in URLForCluster", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			enabled := false
			clusterObject.Spec.Security = &opensearchv1.Security{
				Tls: &opensearchv1.TlsConfig{
					Http: &opensearchv1.TlsConfigHttp{
						Enabled: &enabled,
					},
				},
			}
			clusterObject.Spec.General.ServiceName = "opensearch"
			clusterObject.Namespace = "default"
			clusterObject.Spec.General.HttpPort = 9200

			actualUrl := URLForCluster(&clusterObject)
			Expect(actualUrl).To(ContainSubstring("http://"))
			Expect(actualUrl).NotTo(ContainSubstring("https://"))
		})

		It("should use http protocol in probe commands", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			enabled := false
			clusterObject.Spec.Security = &opensearchv1.Security{
				Tls: &opensearchv1.TlsConfig{
					Http: &opensearchv1.TlsConfigHttp{
						Enabled: &enabled,
					},
				},
			}
			nodePool := opensearchv1.NodePool{
				Component: "masters",
				Roles:     []string{"cluster_manager"},
			}
			result := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(result.Spec.Template.Spec.Containers[0].StartupProbe.ProbeHandler.Exec.Command).
				To(Equal([]string{"/bin/bash", "-c", "curl -k -u \"$(cat /mnt/admin-credentials/username):$(cat /mnt/admin-credentials/password)\" --silent --fail 'http://localhost:9200'"}))
			Expect(result.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.Exec.Command).
				To(Equal([]string{"/bin/bash", "-c", "curl -k -u \"$(cat /mnt/admin-credentials/username):$(cat /mnt/admin-credentials/password)\" --silent --fail 'http://localhost:9200'"}))
		})

		It("should use http scheme in ServiceMonitor", func() {
			clusterObject := ClusterDescWithVersion("2.7.0")
			clusterObject.Name = "test-cluster"
			clusterObject.Namespace = "default"
			enabled := false
			clusterObject.Spec.Security = &opensearchv1.Security{
				Tls: &opensearchv1.TlsConfig{
					Http: &opensearchv1.TlsConfigHttp{
						Enabled: &enabled,
					},
				},
			}
			clusterObject.Spec.General.Monitoring.Enable = true
			clusterObject.Spec.General.Monitoring.ScrapeInterval = "30s"

			result := NewServiceMonitor(&clusterObject)
			Expect(result.Spec.Endpoints[0].Scheme).To(Equal("http"))
		})
	})

	When("Configuring InitHelper Resources", func() {
		It("should propagate Resources to all init containers", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.InitHelper = opensearchv1.InitHelperConfig{
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
			nodePoolSts := NewSTSForNodePool("foobar", &clusterObject, opensearchv1.NodePool{}, "foobar", nil, nil)
			for _, container := range nodePoolSts.Spec.Template.Spec.InitContainers {
				Expect(container.Resources).To(Equal(clusterObject.Spec.InitHelper.Resources))
			}
			bootstrapPod := NewBootstrapPod(&clusterObject, nil, nil)
			for _, container := range bootstrapPod.Spec.InitContainers {
				Expect(container.Resources).To(Equal(clusterObject.Spec.InitHelper.Resources))
			}
		})
	})

	When("Configuring Security Config UpdateJob Resources", func() {
		It("should propagate Resources to the Security Config UpdateJob", func() {
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Spec.Security = &opensearchv1.Security{
				Config: &opensearchv1.SecurityConfig{
					UpdateJob: opensearchv1.SecurityUpdateJobConfig{
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
			clusterObject.Spec.Security = &opensearchv1.Security{
				Config: &opensearchv1.SecurityConfig{
					UpdateJob: opensearchv1.SecurityUpdateJobConfig{
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

	When("configuring a host alias for the cluster", func() {
		It("should configure the host alias for the statefulset and bootstrap pods", func() {
			hostNames := []string{"dummy.com"}
			hostAlias := corev1.HostAlias{
				IP:        "3.5.7.9",
				Hostnames: hostNames,
			}
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Namespace = "foobar"
			clusterObject.Name = "foobar"
			clusterObject.Spec.General.HostAliases = []corev1.HostAlias{hostAlias}
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(sts.Spec.Template.Spec.HostAliases).To(Equal([]corev1.HostAlias{hostAlias}))

			pod := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(pod.Spec.HostAliases).To(Equal([]corev1.HostAlias{hostAlias}))
		})
		It("should overwrite the host alias for the bootstrap pods", func() {
			hostNames := []string{"dummy.com"}
			hostAlias := corev1.HostAlias{
				IP:        "3.5.7.9",
				Hostnames: hostNames,
			}
			bootstrapHostNames := []string{"bootstrap.dummy.com"}
			bootstrapHostAlias := corev1.HostAlias{
				IP:        "3.5.7.10",
				Hostnames: bootstrapHostNames,
			}
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Namespace = "foobar"
			clusterObject.Name = "foobar"
			clusterObject.Spec.General.HostAliases = []corev1.HostAlias{hostAlias}
			clusterObject.Spec.Bootstrap.HostAliases = []corev1.HostAlias{bootstrapHostAlias}
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(sts.Spec.Template.Spec.HostAliases).To(Equal([]corev1.HostAlias{hostAlias}))

			pod := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(pod.Spec.HostAliases).To(Equal([]corev1.HostAlias{bootstrapHostAlias}))
		})
		It("should set the host alias for the bootstrap pods without hostAlias defined in opensearch pods", func() {
			bootstrapHostNames := []string{"bootstrap.dummy.com"}
			bootstrapHostAlias := corev1.HostAlias{
				IP:        "3.5.7.10",
				Hostnames: bootstrapHostNames,
			}
			clusterObject := ClusterDescWithVersion("2.2.1")
			clusterObject.Namespace = "foobar"
			clusterObject.Name = "foobar"
			clusterObject.Spec.Bootstrap.HostAliases = []corev1.HostAlias{bootstrapHostAlias}
			nodePool := opensearchv1.NodePool{
				Replicas:  3,
				Component: "masters",
				Roles:     []string{"cluster_manager", "data"},
			}
			clusterObject.Spec.NodePools = append(clusterObject.Spec.NodePools, nodePool)

			sts := NewSTSForNodePool("foobar", &clusterObject, nodePool, "foobar", nil, nil)
			Expect(sts.Spec.Template.Spec.HostAliases).To(BeNil())

			pod := NewBootstrapPod(&clusterObject, nil, nil)
			Expect(pod.Spec.HostAliases).To(Equal([]corev1.HostAlias{bootstrapHostAlias}))
		})
	})
})
