package util

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Additional volumes", func() {
	namespace := "Additional volume test"
	var volumeConfigs []opensearchv1.AdditionalVolume
	var mockClient *k8s.MockK8sClient

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		mockClient.EXPECT().Context().Return(context.Background())
		volumeConfigs = []opensearchv1.AdditionalVolume{
			{
				Name: "myVolume",
				Path: "myPath/a/b",
			},
		}
	})

	When("configmap is added with subPath", func() {
		It("subPath is set", func() {
			volumeConfigs[0].ConfigMap = &v1.ConfigMapVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(Equal("c"))
		})
	})

	When("configmap is added without subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].ConfigMap = &v1.ConfigMapVolumeSource{}

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("secret is added with subPath", func() {
		It("subPath is set", func() {
			volumeConfigs[0].Secret = &v1.SecretVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(Equal("c"))
		})
	})

	When("secret is added without subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].Secret = &v1.SecretVolumeSource{}

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("emptyDir is added with subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].EmptyDir = &v1.EmptyDirVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("emptyDir is added without subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].EmptyDir = &v1.EmptyDirVolumeSource{}

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("CSI readOnly volume is added", func() {
		It("Should have CSIVolumeSource fields", func() {
			readOnly := true
			volumeConfigs[0].CSI = &v1.CSIVolumeSource{
				Driver:   "testDriver",
				ReadOnly: &readOnly,
				VolumeAttributes: map[string]string{
					"secretProviderClass": "testSecretProviderClass",
				},
				NodePublishSecretRef: &v1.LocalObjectReference{
					Name: "testSecret",
				},
			}

			volume, _, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].CSI.Driver).To(Equal("testDriver"))
			Expect(*volume[0].CSI.ReadOnly).Should(BeTrue())
			Expect(volume[0].CSI.VolumeAttributes["secretProviderClass"]).To(Equal("testSecretProviderClass"))
			Expect(volume[0].CSI.NodePublishSecretRef.Name).To(Equal("testSecret"))
		})
	})

	When("CSI read-write volume is added", func() {
		It("Should have CSIVolumeSource fields", func() {
			readOnly := false
			volumeConfigs[0].CSI = &v1.CSIVolumeSource{
				Driver:   "testDriver",
				ReadOnly: &readOnly,
				VolumeAttributes: map[string]string{
					"secretProviderClass": "testSecretProviderClass",
				},
				NodePublishSecretRef: &v1.LocalObjectReference{
					Name: "testSecret",
				},
			}

			volume, _, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].CSI.Driver).To(Equal("testDriver"))
			Expect(*volume[0].CSI.ReadOnly).Should(BeFalse())
			Expect(volume[0].CSI.VolumeAttributes["secretProviderClass"]).To(Equal("testSecretProviderClass"))
			Expect(volume[0].CSI.NodePublishSecretRef.Name).To(Equal("testSecret"))
		})
	})

	When("CSI volume is added with subPath", func() {
		It("Should have the subPath", func() {
			volumeConfigs[0].CSI = &v1.CSIVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(Equal("c"))
		})
	})

	When("CSI volume is added without subPath", func() {
		It("Should not have the subPath", func() {
			volumeConfigs[0].CSI = &v1.CSIVolumeSource{}

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("PersistentVolumeClaim volume is added", func() {
		It("Should have PersistentVolumeClaimVolumeSource fields", func() {
			readOnly := true
			volumeConfigs[0].PersistentVolumeClaim = &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: "testClaim",
				ReadOnly:  readOnly,
			}

			volume, _, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].PersistentVolumeClaim.ClaimName).To(Equal("testClaim"))
			Expect(volume[0].PersistentVolumeClaim.ReadOnly).Should(BeTrue())
		})
	})

	When("Projected volume is added", func() {
		It("Should have ProjectedVolumeSource fields", func() {
			volumeConfigs[0].Projected = &v1.ProjectedVolumeSource{
				Sources: []v1.VolumeProjection{},
			}

			volume, _, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].Projected.Sources).To(BeEmpty())
		})
	})

	When("Projected volume is added with a ServiceAccountToken source", func() {
		It("Should have Path set on the volume source", func() {
			volumeConfigs[0].Projected = &v1.ProjectedVolumeSource{
				Sources: []v1.VolumeProjection{{
					ServiceAccountToken: &v1.ServiceAccountTokenProjection{
						Path: "token",
					},
				}},
			}

			volume, _, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].Projected.Sources[0].ServiceAccountToken.Path).To(Equal("token"))
		})
	})

	When("Projected volume is added with subPath", func() {
		It("Should have the subPath", func() {
			volumeConfigs[0].Projected = &v1.ProjectedVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(Equal("c"))
		})
	})

	When("Projected volume is added without subPath", func() {
		It("Should not have the subPath", func() {
			volumeConfigs[0].Projected = &v1.ProjectedVolumeSource{}

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("NFS volume is added", func() {
		It("Should have NFSVolumeSource fields and mount readOnly", func() {
			volumeConfigs[0].NFS = &v1.NFSVolumeSource{
				Server:   "10.0.0.1",
				Path:     "/export/path",
				ReadOnly: true,
			}

			volume, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].NFS.Server).To(Equal("10.0.0.1"))
			Expect(volume[0].NFS.Path).To(Equal("/export/path"))
			Expect(volume[0].NFS.ReadOnly).To(BeTrue())
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].ReadOnly).To(BeTrue())
			Expect(volumeMount[0].SubPath).To(BeEmpty())

		})
	})

	When("HostPath volume is added", func() {
		It("Should have HostPathVolumeSource fields and mount read-write", func() {
			hostPathType := v1.HostPathDirectoryOrCreate
			volumeConfigs[0].HostPath = &v1.HostPathVolumeSource{
				Path: "/host/path",
				Type: &hostPathType,
			}

			volume, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volume[0].HostPath.Path).To(Equal("/host/path"))
			Expect(*volume[0].HostPath.Type).To(Equal(v1.HostPathDirectoryOrCreate))
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].ReadOnly).To(BeFalse())
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("HostPath volume is added with subPath", func() {
		It("Should not set subPath", func() {
			hostPathType := v1.HostPathDirectory
			volumeConfigs[0].HostPath = &v1.HostPathVolumeSource{
				Path: "/host/path",
				Type: &hostPathType,
			}
			volumeConfigs[0].SubPath = "subpath"

			_, volumeMount, _, _ := CreateAdditionalVolumes(mockClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})
})

var _ = Describe("OpensearchClusterURL", func() {
	When("HTTP TLS is enabled", func() {
		It("should return https URL", func() {
			enabled := true
			cluster := &opensearchv1.OpenSearchCluster{
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: "test-service",
						HttpPort:    9200,
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Http: &opensearchv1.TlsConfigHttp{
								Enabled: &enabled,
							},
						},
					},
				},
			}
			cluster.Name = "test-cluster"
			cluster.Namespace = "test-namespace"

			url := OpensearchClusterURL(cluster)
			Expect(url).To(ContainSubstring("https://"))
			Expect(url).To(ContainSubstring("test-service"))
			Expect(url).To(ContainSubstring("test-namespace"))
			Expect(url).To(ContainSubstring(":9200"))
		})
	})

	When("HTTP TLS is disabled", func() {
		It("should return http URL", func() {
			enabled := false
			cluster := &opensearchv1.OpenSearchCluster{
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: "test-service",
						HttpPort:    9200,
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Http: &opensearchv1.TlsConfigHttp{
								Enabled: &enabled,
							},
						},
					},
				},
			}
			cluster.Name = "test-cluster"
			cluster.Namespace = "test-namespace"

			url := OpensearchClusterURL(cluster)
			Expect(url).To(ContainSubstring("http://"))
			Expect(url).NotTo(ContainSubstring("https://"))
			Expect(url).To(ContainSubstring("test-service"))
			Expect(url).To(ContainSubstring("test-namespace"))
			Expect(url).To(ContainSubstring(":9200"))
		})
	})
})
