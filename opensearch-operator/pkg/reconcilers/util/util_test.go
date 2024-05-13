package util

import (
	"context"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Additional volumes", func() {
	namespace := "Additional volume test"
	var volumeConfigs []opsterv1.AdditionalVolume
	var mockClient *k8s.MockK8sClient

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		mockClient.EXPECT().Context().Return(context.Background())
		volumeConfigs = []opsterv1.AdditionalVolume{
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

	When("CSI volume is added", func() {
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
})
