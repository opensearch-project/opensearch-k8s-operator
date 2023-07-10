package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Cluster Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "autoscaler-test-autoscaler"
		namespace   = "default"
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		Autoscaler = ComposeAutoscalerCrd(clusterName, namespace)
	)

	/// ------- Creation Check phase -------

	When("Creating a Autoscaler CRD instance", func() {
		It("Should create the autoscaler if ns exists", func() {
			Expect(k8sClient.Create(context.Background(), &Autoscaler)).To(Succeed())
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      clusterName,
					Namespace: namespace,
				}, &Autoscaler)
			}, timeout, interval).Should(Succeed())
		})
	})
})
