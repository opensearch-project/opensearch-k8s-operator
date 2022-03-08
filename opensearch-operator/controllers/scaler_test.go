package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Scaler Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-nodes"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(clusterName, namespace)
		nodePool          = appsv1.StatefulSet{}
		cluster2          = opsterv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	Context("When create OpenSearch CRD - nodes", func() {
		It("Should create the namespace first", func() {
			Expect(CreateNamespace(k8sClient, &OpensearchCluster)).Should(Succeed())
			By("Create cluster ns ")
			Eventually(func() bool {
				return IsNsCreated(k8sClient, namespace)
			}, timeout, interval).Should(BeTrue())
		})

		It("should apply the cluster instance successfully", func() {
			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("When changing Opensearch NodePool Replicas", func() {
		It("should add a new status about the operation", func() {
			By("Wait for cluster instance to be created")
			Eventually(func() bool {
				return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &OpensearchCluster) == nil
			}, time.Second*10, interval).Should(BeTrue())
			By("Update replicas")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &OpensearchCluster); err != nil {
					return err
				}
				OpensearchCluster.Spec.NodePools[0].Replicas = 2

				return k8sClient.Update(context.Background(), &OpensearchCluster)
			})
			Expect(err).ToNot(HaveOccurred())
			status := len(OpensearchCluster.Status.ComponentsStatus)

			By("Check ComponentsStatus")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &cluster2); err != nil {
					return false
				}
				return status != len(cluster2.Status.ComponentsStatus)
			}, time.Second*60, 30*time.Millisecond).Should(BeFalse())
		})
	})

	Context("When changing CRD nodepool replicas", func() {
		It("should implement new number of replicas to the cluster", func() {
			By("check replicas")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: clusterName + "-" + cluster2.Spec.NodePools[0].Component}, &nodePool); err != nil {
					return false
				}
				if pointer.Int32Deref(nodePool.Spec.Replicas, 1) != 2 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Deletion Check phase -------

	Context("When deleting OpenSearch CRD ", func() {
		It("should set correct owner references", func() {
			for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
				nodePool := appsv1.StatefulSet{}
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool)).To(Succeed())
				Expect(HasOwnerReference(&nodePool, &OpensearchCluster)).To(BeTrue())
			}
		})
	})

})
