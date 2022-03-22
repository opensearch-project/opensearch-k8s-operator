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
				for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: fmt.Sprintf("%s-%s", OpensearchCluster.Spec.General.ServiceName, nodePoolSpec.Component)}, &service); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("should apply the right cluster resources successfully", func() {
			for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
				Expect(nodePoolSpec.Resources.Limits.Cpu()).Should(Equal("500m"))
				Expect(nodePoolSpec.Resources.Limits.Memory()).Should(Equal("2Gi"))
			}
		})

		It("should set correct owner references", func() {
			service := corev1.Service{}
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName}, &service)).To(Succeed())
			Expect(HasOwnerReference(&service, &OpensearchCluster)).To(BeTrue())
			for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
				nodePool := appsv1.StatefulSet{}
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool)).To(Succeed())
				Expect(HasOwnerReference(&nodePool, &OpensearchCluster)).To(BeTrue())
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-" + nodePoolSpec.Component}, &service)).To(Succeed())
				Expect(HasOwnerReference(&service, &OpensearchCluster)).To(BeTrue())
			}
		})
	})

	/// ------- Tests nodepool cleanup -------
	Context("When updating an OpensearchCluster kind instance", func() {
		It("should remove old node pools", func() {
			// Fetch the latest version of the opensearch object
			Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster)).Should(Succeed())

			// Update the opensearch object
			OpensearchCluster.Spec.NodePools = OpensearchCluster.Spec.NodePools[:2]
			Expect(k8sClient.Update(context.Background(), &OpensearchCluster)).Should(Succeed())

			Eventually(func() bool {
				stsList := &appsv1.StatefulSetList{}
				err := k8sClient.List(context.Background(), stsList, client.InNamespace(OpensearchCluster.Name))
				if err != nil {
					return false
				}

				return len(stsList.Items) == 2
			})
		})
	})
})
