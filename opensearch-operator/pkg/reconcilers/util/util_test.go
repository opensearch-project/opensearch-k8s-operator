package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	opsterv1 "opensearch.opster.io/api/v1"
)

var _ = Describe("Additional volumes", func() {
	var namespace = "Additional volume test"
	var volumeConfigs []opsterv1.AdditionalVolume

	BeforeEach(func() {
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

			var _, volumeMount, _, _ = CreateAdditionalVolumes(ctx, k8sClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(Equal("c"))
		})
	})

	When("configmap is added without subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].ConfigMap = &v1.ConfigMapVolumeSource{}

			var _, volumeMount, _, _ = CreateAdditionalVolumes(ctx, k8sClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("secret is added with subPath", func() {
		It("subPath is set", func() {
			volumeConfigs[0].Secret = &v1.SecretVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			var _, volumeMount, _, _ = CreateAdditionalVolumes(ctx, k8sClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(Equal("c"))
		})
	})

	When("secret is added without subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].Secret = &v1.SecretVolumeSource{}

			var _, volumeMount, _, _ = CreateAdditionalVolumes(ctx, k8sClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("emptyDir is added with subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].EmptyDir = &v1.EmptyDirVolumeSource{}
			volumeConfigs[0].SubPath = "c"

			var _, volumeMount, _, _ = CreateAdditionalVolumes(ctx, k8sClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})

	When("emptyDir is added without subPath", func() {
		It("subPath is not set", func() {
			volumeConfigs[0].EmptyDir = &v1.EmptyDirVolumeSource{}

			var _, volumeMount, _, _ = CreateAdditionalVolumes(ctx, k8sClient, namespace, volumeConfigs)
			Expect(volumeMount[0].MountPath).To(Equal("myPath/a/b"))
			Expect(volumeMount[0].SubPath).To(BeEmpty())
		})
	})
})
