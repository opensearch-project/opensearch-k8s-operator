package controllers

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
	//+kubebuilder:scaffold:imports
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

	key := types.NamespacedName{
		Name:      "cluster-test",
		Namespace: "default",
	}
	Context("When createing a OS kind Instance", func() {
		It("should create a new os cluster ", func() {

			OpensearchCluster := ComposeOpensearchCrd(ClusterName, ClusterNameSpaces)

			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Expecting submitted")
			Eventually(func() bool {
				cm := corev1.ConfigMap{}
				sts := sts.StatefulSet{}
				service := corev1.Service{}
				key_resource := key
				key_resource.Name = "cluster-test" + OpensearchCluster.Spec.NodePools[0].Component
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterNameSpaces, Name: "opensearch-yml"}, &cm); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterNameSpaces, Name: "os-dash"}, &cm); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterNameSpaces, Name: OpensearchCluster.Spec.General.ServiceName + "-svc"}, &service); err != nil {
					return false
				}
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterNameSpaces, Name: OpensearchCluster.Spec.General.ServiceName + "-headleass-service"}, &service); err != nil {
					return false
				}
				for i := 0; i < len(OpensearchCluster.Spec.NodePools); i++ {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ClusterNameSpaces, Name: ClusterNameSpaces + "-" + OpensearchCluster.Spec.NodePools[i].Component}, &sts); err != nil {
						return false
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
})
