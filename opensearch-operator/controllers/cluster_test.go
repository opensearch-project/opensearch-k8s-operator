package controllers

import (
	"context"
	"fmt"
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

var _ = Describe("OpensearchCluster Controller", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName       = "cluster-test-cluster"
		ClusterNameSpaces = "default"
		timeout           = time.Second * 30
		interval          = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(ClusterName, ClusterNameSpaces)
		nodePool          = sts.StatefulSet{}
		service           = corev1.Service{}
		//deploy            = sts.Deployment{}
		//cluster           = opsterv1.OpenSearchCluster{}
		//cluster2 = opsterv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	ns := ComposeNs(ClusterName)
	Context("When create OpenSearch CRD instance", func() {
		It("should create cluster NS and CRD instance", func() {

			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())
			fmt.Println(OpensearchCluster)

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

	Context("When creating a OpenSearchCluster kind Instance", func() {
		It("should create a new opensearch cluster ", func() {

			By("Opensearch cluster")
			Eventually(func() bool {

				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName}, &service); err != nil {
					return false
				}
				for _, name := range []string{"master", "nodes", "client"} {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: fmt.Sprintf("%s-%s", OpensearchCluster.Spec.General.ServiceName, name)}, &service); err != nil {
						return false
					}
				}
				for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-" + nodePoolSpec.Component}, &nodePool); err != nil {
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

			Expect(k8sClient.Delete(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Delete cluster ns ")
			Eventually(func() bool {
				return IsNsDeleted(k8sClient, ns)
			}, timeout, interval).Should(BeTrue())
		})
	})

})
