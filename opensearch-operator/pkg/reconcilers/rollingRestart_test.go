package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
)

var _ = Describe("RollingRestart Reconciler", func() {
	Describe("hasManagerRole", func() {
		Context("with cluster_manager role", func() {
			It("should return true", func() {
				nodePool := opsterv1.NodePool{
					Component: "master",
					Roles:     []string{"cluster_manager"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with master role", func() {
			It("should return true", func() {
				nodePool := opsterv1.NodePool{
					Component: "master",
					Roles:     []string{"master"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with data role only", func() {
			It("should return false", func() {
				nodePool := opsterv1.NodePool{
					Component: "data",
					Roles:     []string{"data"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeFalse())
			})
		})

		Context("with no roles", func() {
			It("should return false", func() {
				nodePool := opsterv1.NodePool{
					Component: "coordinating",
					Roles:     []string{},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeFalse())
			})
		})
	})

	Describe("hasDataRole", func() {
		Context("with data role", func() {
			It("should return true", func() {
				nodePool := opsterv1.NodePool{
					Component: "data",
					Roles:     []string{"data"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with cluster_manager role only", func() {
			It("should return false", func() {
				nodePool := opsterv1.NodePool{
					Component: "master",
					Roles:     []string{"cluster_manager"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeFalse())
			})
		})

		Context("with multiple roles including data", func() {
			It("should return true", func() {
				nodePool := opsterv1.NodePool{
					Component: "coordinating",
					Roles:     []string{"data", "ingest"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with no roles", func() {
			It("should return false", func() {
				nodePool := opsterv1.NodePool{
					Component: "coordinating",
					Roles:     []string{},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeFalse())
			})
		})
	})
})
