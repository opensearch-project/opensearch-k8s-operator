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

var _ = Describe("OpensearchCluster Controller", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName       = "cluster-test-dash"
		ClusterNameSpaces = "default"
		timeout           = time.Second * 30
		interval          = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(ClusterName, ClusterNameSpaces)
		cm                = corev1.ConfigMap{}
		//	nodePool          = sts.StatefulSet{}
		service = corev1.Service{}
		deploy  = sts.Deployment{}
		//cluster           = opsterv1.OpenSearchCluster{}
		//cluster2 = opsterv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	ns := ComposeNs(ClusterName)
	Context("When create OpenSearch CRD - dash", func() {
		It("should create cluster NS", func() {
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

	Context("When createing a OpenSearchCluster kind Instance - and Dashboard is Enable", func() {
		It("should create all Opensearch-dashboard resources", func() {
			//fmt.Println(OpensearchCluster)
			fmt.Println("\n DAShBOARD - START")

			By("Opensearch Dashboard")
			Eventually(func() bool {
				fmt.Println("\n DAShBOARD - START - 2")
				//// -------- Dashboard tests ---------
				if OpensearchCluster.Spec.Dashboards.Enable {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-dashboards"}, &deploy); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: ClusterName + "-dashboards-config"}, &cm); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-dashboards"}, &service); err != nil {
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
				fmt.Println("\n check ns dashboard")
				return IsNsDeleted(k8sClient, ns)
			}, timeout, interval).Should(BeTrue())
		})
	})

})
