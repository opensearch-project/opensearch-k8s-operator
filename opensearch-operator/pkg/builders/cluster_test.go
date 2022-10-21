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
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			var clusterObject = ClusterDescWithversion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			var result = NewSTSForNodePool("foobar", &clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:1.27.2-buildx"))
		})
		It("should use InitHelper.Image as InitHelper image if configured", func() {
			var clusterObject = ClusterDescWithversion("2.2.1")
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
			var clusterObject = ClusterDescWithversion("2.2.1")
			var result = NewSTSForNodePool("foobar", &clusterObject, opsterv1.NodePool{}, "foobar", nil, nil, nil)
			Expect(result.Spec.Template.Spec.InitContainers[0].Image).To(Equal("public.ecr.aws/opsterio/busybox:1.27.2-buildx"))
		})
	})

	When("Constructing a bootstrap pod", func() {
		It("should use General.DefaultRepo for the InitHelper image if configured", func() {
			var clusterObject = ClusterDescWithversion("2.2.1")
			customRepository := "mycustomrepo.cr"
			clusterObject.Spec.General.DefaultRepo = &customRepository
			var result = NewBootstrapPod(&clusterObject, nil, nil)
			Expect(result.Spec.InitContainers[0].Image).To(Equal("mycustomrepo.cr/busybox:1.27.2-buildx"))
		})
	})
})
