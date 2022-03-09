package controllers

import (
	"context"
	"fmt"
	"time"

	sts "k8s.io/api/apps/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Dashboards Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-dash"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(clusterName, namespace)
		cm                = corev1.ConfigMap{}
		service           = corev1.Service{}
		deploy            = sts.Deployment{}
	)

	/// ------- Creation Check phase -------

	Context("When create OpenSearch CRD - dash", func() {
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

	Context("When createing a OpenSearchCluster kind Instance - and Dashboard is Enable", func() {
		It("should create all Opensearch-dashboard resources", func() {
			//fmt.Println(OpensearchCluster)
			fmt.Println("\n DAShBOARD - START")

			By("Opensearch Dashboard")
			Eventually(func() bool {
				fmt.Println("\n DAShBOARD - START - 2")
				//// -------- Dashboard tests ---------
				if OpensearchCluster.Spec.Dashboards.Enable {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-dashboards"}, &deploy); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-dashboards-config"}, &cm); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-dashboards"}, &service); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
		It("should set correct owner references", func() {
			Expect(HasOwnerReference(&deploy, &OpensearchCluster)).To(BeTrue())
			Expect(HasOwnerReference(&cm, &OpensearchCluster)).To(BeTrue())
			Expect(HasOwnerReference(&service, &OpensearchCluster)).To(BeTrue())
		})
	})
})
