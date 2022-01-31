package controllers

import (
	"context"
	opsterv1 "opensearch.opster.io/api/v1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
	//banzi "github.com/banzaicloud/operator-tools/pkg/reconciler"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("OpensearchCLuster Controller", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName       = "cluster-test"
		ClusterNameSpaces = "default"
		timeout           = time.Second * 30
		interval          = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(ClusterName, ClusterNameSpaces)
		cm                = corev1.ConfigMap{}
		nodePool          = sts.StatefulSet{}
		service           = corev1.Service{}
		deploy            = sts.Deployment{}
		//cluster           = opsterv1.OpenSearchCluster{}
		cluster2 = opsterv1.OpenSearchCluster{}
	)

	ns := ComposeNs(ClusterNameSpaces)

	Context("When createing a OpenSearchCluster kind Instance", func() {
		It("should create a new opensearch cluster ", func() {

			//	Expect(k8sClient.Create(context.Background(), &ns)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Opensearch cluster")
			Eventually(func() bool {

				if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: ClusterName}, &ns); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: "opensearch-yml"}, &cm); err != nil {
					return false
				}

				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-svc"}, &service); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-headless-service"}, &service); err != nil {
					return false
				}
				for i := 0; i < len(OpensearchCluster.Spec.NodePools); i++ {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-" + OpensearchCluster.Spec.NodePools[i].Component}, &nodePool); err != nil {
						return false
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When createing a OpenSearchCluster kind Instance - and Dashboard is Enable", func() {
		It("should create all Opensearch-dashboard resources", func() {

			By("Opensearch Dashboard")
			Eventually(func() bool {
				//// -------- Dashboard tests ---------
				if OpensearchCluster.Spec.Dashboards.Enable {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-dashboards"}, &deploy); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: "opensearch-dashboards"}, &cm); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-dashboards-svc"}, &service); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When changing Opensearch NodePool Replicas", func() {
		It("should to add new status about the operation", func() {

			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: OpensearchCluster.Name}, &OpensearchCluster)).Should(Succeed())

			newRep := OpensearchCluster.Spec.NodePools[0].Replicas - 1
			OpensearchCluster.Spec.NodePools[0].Replicas = newRep

			status := len(OpensearchCluster.Status.ComponentsStatus)
			Expect(k8sClient.Update(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Statuses checker ")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: OpensearchCluster.Name}, &cluster2); err != nil {
					return false
				}
				newStatuss := len(cluster2.Status.ComponentsStatus)
				return status != newStatuss
			}, timeout, 30*time.Millisecond).Should(BeTrue())
		})
	})

	Context("When changing CRD nodepool replicas", func() {
		It("should implement new number of replicas to the cluster", func() {

			By("check replicas")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-" + cluster2.Spec.NodePools[0].Component}, &nodePool); err != nil {
					return false
				}

				if *nodePool.Spec.Replicas != cluster2.Spec.NodePools[0].Replicas {

					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When deleting OpenSearch CRD ", func() {
		It("should delete cluster NS and resources", func() {

			Expect(k8sClient.Delete(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Delete cluster ns ")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: ClusterName}, &ns); err == nil {
					return ns.Status.Phase == "Terminating"
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

})
