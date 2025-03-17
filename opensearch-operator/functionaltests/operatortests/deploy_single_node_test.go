package operatortests

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DeploySingleNode", Ordered, func() {
	name := "deploy-single-node"
	namespace := "default"

	BeforeAll(func() {
		CreateKubernetesObjects(name)
	})

	When("creating a cluster", Ordered, func() {
		It("should have 1 ready master pod", func() {
			sts := appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-masters", Namespace: namespace}, &sts)
				if err == nil {
					return sts.Status.ReadyReplicas
				}
				return 0
			}, time.Minute*15, time.Second*5).Should(Equal(int32(1)))
		})
	})

	AfterAll(func() {
		Cleanup(name)
	})
})
