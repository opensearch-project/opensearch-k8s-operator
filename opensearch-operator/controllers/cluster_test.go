package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Cluster Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-cluster"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(clusterName, namespace)
		nodePool          = appsv1.StatefulSet{}
		service           = corev1.Service{}
	)

	/// ------- Creation Check phase -------

	Context("When creating a OpenSearch CRD instance", func() {
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

	Context("When creating a OpenSearchCluster kind Instance", func() {
		It("should create a new opensearch cluster ", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName}, &service); err != nil {
					return false
				}
				for _, name := range []string{"master", "nodes", "client"} {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: fmt.Sprintf("%s-%s", OpensearchCluster.Spec.General.ServiceName, name)}, &service); err != nil {
						return false
					}
				}
				for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Deletion Check phase -------

	Context("When deleting OpenSearch CRD ", func() {
		It("should delete cluster resources", func() {
			Expect(k8sClient.Delete(context.Background(), &OpensearchCluster)).Should(Succeed())

			Eventually(func() bool {
				for _, nodePool := range OpensearchCluster.Spec.NodePools {
					if !IsSTSDeleted(k8sClient, clusterName+"-"+nodePool.Component, clusterName) {
						return false
					}
				}
				return IsServiceDeleted(k8sClient, OpensearchCluster.Spec.General.ServiceName, clusterName)
			}, timeout, interval).Should(BeTrue())
		})
	})

})
