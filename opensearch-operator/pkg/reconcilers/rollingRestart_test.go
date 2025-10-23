package reconcilers

import (
	"context"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RollingRestart Reconciler", func() {
	var (
		mockClient *k8s.MockK8sClient
		instance   *opsterv1.OpenSearchCluster
		reconciler *RollingRestartReconciler
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		instance = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					ServiceName: "test-cluster",
					Version:     "2.14.0",
				},
			},
			Status: opsterv1.ClusterStatus{
				Initialized: true,
			},
		}
		reconciler = &RollingRestartReconciler{
			client:   mockClient,
			ctx:      context.Background(),
			instance: instance,
		}
	})

	Describe("groupNodePoolsByRole", func() {
		Context("with mixed roles across availability zones", func() {
			BeforeEach(func() {
				instance.Spec.NodePools = []opsterv1.NodePool{
					{
						Component: "master-a",
						Replicas:  1,
						Roles:     []string{"cluster_manager"},
					},
					{
						Component: "master-b",
						Replicas:  1,
						Roles:     []string{"cluster_manager"},
					},
					{
						Component: "master-c",
						Replicas:  1,
						Roles:     []string{"cluster_manager"},
					},
					{
						Component: "data-a",
						Replicas:  2,
						Roles:     []string{"data"},
					},
					{
						Component: "data-b",
						Replicas:  2,
						Roles:     []string{"data"},
					},
					{
						Component: "coordinating",
						Replicas:  2,
						Roles:     []string{"data", "ingest"},
					},
				}
			})

			It("should correctly group node pools by role", func() {
				groups := reconciler.groupNodePoolsByRole()

				Expect(groups).NotTo(BeNil())
				Expect(groups).To(HaveKey("masterOnly"))
				Expect(groups).To(HaveKey("dataOnly"))
				Expect(groups).To(HaveKey("dataAndMaster"))
				Expect(groups).To(HaveKey("other"))

				// Check master-only pools
				Expect(groups["masterOnly"]).To(HaveLen(3))
				Expect(groups["masterOnly"][0].Component).To(Equal("master-a"))
				Expect(groups["masterOnly"][1].Component).To(Equal("master-b"))
				Expect(groups["masterOnly"][2].Component).To(Equal("master-c"))

				// Check data-only pools
				Expect(groups["dataOnly"]).To(HaveLen(3))
				Expect(groups["dataOnly"][0].Component).To(Equal("data-a"))
				Expect(groups["dataOnly"][1].Component).To(Equal("data-b"))

				// Check data+master pools
				Expect(groups["dataAndMaster"]).To(HaveLen(0))
			})
		})
	})

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
