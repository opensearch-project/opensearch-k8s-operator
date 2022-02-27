package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sts "k8s.io/api/apps/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("OpensearchCLuster Controller", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName = "cluster-test-nodes"
		NameSpace   = "default"
		timeout     = time.Second * 120
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(ClusterName, NameSpace)
		nodePool          = sts.StatefulSet{}
		cluster2          = opsterv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	ns := ComposeNs(ClusterName)
	Context("When create OpenSearch CRD - nodes", func() {
		It("should create cluster NS and CRD instance", func() {
			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())
			By("Create cluster ns ")
			Eventually(func() bool {
				if !IsNsCreated(k8sClient, ns) {
					return false
				}
				if !IsClusterCreated(k8sClient, OpensearchCluster) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("When changing Opensearch NodePool Replicas", func() {
		It("should to add new status about the operation", func() {

			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: OpensearchCluster.Name}, &OpensearchCluster)).Should(Succeed())

			newRep := OpensearchCluster.Spec.NodePools[0].Replicas - 1
			OpensearchCluster.Spec.NodePools[0].Replicas = newRep

			status := len(OpensearchCluster.Status.ComponentsStatus)
			Expect(k8sClient.Update(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("ComponentsStatus checker ")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: OpensearchCluster.Name}, &cluster2); err != nil {
					return false
				}
				newStatuss := len(cluster2.Status.ComponentsStatus)
				return status != newStatuss
			}, time.Second*60, 30*time.Millisecond).Should(BeFalse())
		})
	})

	Context("When changing CRD nodepool replicas", func() {
		It("should implement new number of replicas to the cluster", func() {
			By("check replicas")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-" + cluster2.Spec.NodePools[0].Component}, &nodePool); err != nil {
					return false
				}
				oldCrd := ComposeOpensearchCrd(ClusterName, NameSpace)
				if *nodePool.Spec.Replicas != oldCrd.Spec.NodePools[0].Replicas {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Deletion Check phase -------

	Context("When deleting OpenSearch CRD ", func() {
		It("should delete cluster NS and resources", func() {

			Expect(k8sClient.Delete(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Delete cluster ns ")
			Eventually(func() bool {
				return IsNsDeleted(k8sClient, ns)
			}, timeout, interval).Should(BeTrue())
		})
	})

})
