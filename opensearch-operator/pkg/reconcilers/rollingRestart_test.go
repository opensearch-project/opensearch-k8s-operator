package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
)

var _ = Describe("RollingRestart Reconciler", func() {
	Describe("hasManagerRole", func() {
		Context("with cluster_manager role", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "master",
					Roles:     []string{"cluster_manager"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with master role", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "master",
					Roles:     []string{"master"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with data role only", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "data",
					Roles:     []string{"data"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeFalse())
			})
		})

		Context("with no roles", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
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
				nodePool := opensearchv1.NodePool{
					Component: "data",
					Roles:     []string{"data"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with cluster_manager role only", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "master",
					Roles:     []string{"cluster_manager"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeFalse())
			})
		})

		Context("with multiple roles including data", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "coordinating",
					Roles:     []string{"data", "ingest"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with no roles", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "coordinating",
					Roles:     []string{},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeFalse())
			})
		})
	})

	Describe("findStatus", func() {
		Context("when ComponentsStatus has a non-RollingRestart entry at index 0", func() {
			It("should return the matched RollingRestart entry, not comp[0]", func() {
				instance := &opensearchv1.OpenSearchCluster{
					Status: opensearchv1.ClusterStatus{
						ComponentsStatus: []opensearchv1.ComponentStatus{
							{Component: "Scaler", Status: statusInProgress},
							{Component: componentName, Status: statusInProgress},
						},
					},
				}
				r := &RollingRestartReconciler{instance: instance}

				found := r.findStatus()

				Expect(found).NotTo(BeNil())
				Expect(found.Component).To(Equal(componentName))
				Expect(found.Status).To(Equal(statusInProgress))
			})
		})

		Context("when there is no RollingRestart entry", func() {
			It("should return nil", func() {
				instance := &opensearchv1.OpenSearchCluster{
					Status: opensearchv1.ClusterStatus{
						ComponentsStatus: []opensearchv1.ComponentStatus{
							{Component: "Scaler", Status: statusInProgress},
						},
					},
				}
				r := &RollingRestartReconciler{instance: instance}

				Expect(r.findStatus()).To(BeNil())
			})
		})
	})
})
