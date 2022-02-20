package controllers

import (
	"context"
	"fmt"
	"opensearch.opster.io/pkg/helpers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("OpensearchCLuster Controller", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName       = "cluster-test-cluster"
		ClusterNameSpaces = "default"
		timeout           = time.Second * 30
		interval          = time.Second * 1
	)
	var (
		OpensearchCluster = helpers.ComposeOpensearchCrd(ClusterName, ClusterNameSpaces)
		cm                = corev1.ConfigMap{}
		nodePool          = sts.StatefulSet{}
		service           = corev1.Service{}
		//deploy            = sts.Deployment{}
		//cluster           = opsterv1.OpenSearchCluster{}
		//cluster2 = opsterv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	ns := helpers.ComposeNs(ClusterName)
	Context("When create OpenSearch CRD instance", func() {
		It("should create cluster NS and CRD instance", func() {

			Expect(helpers.K8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())
			fmt.Println(OpensearchCluster)

			By("Create cluster ns ")
			Eventually(func() bool {
				if !helpers.IsNsCreated(helpers.K8sClient, ns) {
					return false
				}
				if !helpers.IsClusterCreated(helpers.K8sClient, OpensearchCluster) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("When createing a OpenSearchCluster kind Instance", func() {
		It("should create a new opensearch cluster ", func() {

			By("Opensearch cluster")
			Eventually(func() bool {

				if err := helpers.K8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: "opensearch-yml"}, &cm); err != nil {
					return false
				}

				if err := helpers.K8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-svc"}, &service); err != nil {
					return false
				}
				if err := helpers.K8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-headless-service"}, &service); err != nil {
					return false
				}
				for i := 0; i < len(OpensearchCluster.Spec.NodePools); i++ {
					if err := helpers.K8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-" + OpensearchCluster.Spec.NodePools[i].Component}, &nodePool); err != nil {
						return false
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Deletion Check phase -------

	Context("When deleting OpenSearch CRD ", func() {
		It("should delete cluster NS and resources", func() {

			Expect(helpers.K8sClient.Delete(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Delete cluster ns ")
			Eventually(func() bool {
				return helpers.IsNsDeleted(helpers.K8sClient, ns)
			}, timeout, interval).Should(BeTrue())
		})
	})

})
