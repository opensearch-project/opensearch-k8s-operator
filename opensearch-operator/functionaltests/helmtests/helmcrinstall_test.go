package helmtests

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The cluster has been created using Helm outside of this test. This test verifies the presence of the resources after the cluster is created.
var _ = Describe("DeployWithHelm", Ordered, func() {
	name := "opensearch-cluster"
	namespace := "default"

	When("cluster is created using helm", Ordered, func() {
		It("should have 3 ready master pods", func() {
			sts := appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-masters", Namespace: namespace}, &sts)
				if err == nil {
					return sts.Status.ReadyReplicas
				}
				return 0
			}, time.Minute*15, time.Second*5).Should(Equal(int32(3)))
		})

		It("should have a ready dashboards pod", func() {
			deployment := appsv1.Deployment{}
			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-dashboards", Namespace: namespace}, &deployment)
				if err == nil {
					return deployment.Status.ReadyReplicas
				}
				return 0
			}, time.Minute*5, time.Second*5).Should(Equal(int32(1)))
		})
	})
})
