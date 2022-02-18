package controllers

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Configuration Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "configuration-test"
		timeout     = time.Second * 10
		interval    = time.Second * 1
	)

	Context("When Reconciling the configuration controller with no configuration snippets", func() {
		It("should not create a configmap ", func() {
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: clusterName}}}

			underTest := ConfigurationReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				Recorder: &helpers.MockEventRecorder{},
			}
			controllerContext := NewControllerContext()
			_, err := underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())

			configMap := corev1.ConfigMap{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-config", Namespace: clusterName}, &configMap)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When Reconciling the configuration controller with some configuration snippets", func() {
		It("should create a configmap ", func() {
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: clusterName}}}

			underTest := ConfigurationReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				Recorder: &helpers.MockEventRecorder{},
			}
			controllerContext := NewControllerContext()
			controllerContext.AddConfig("foo", "bar")
			controllerContext.AddConfig("bar", "something")
			controllerContext.AddConfig("bar", "baz")
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				configMap := corev1.ConfigMap{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-config", Namespace: clusterName}, &configMap)
				if err != nil {
					return false
				}
				data, exists := configMap.Data["opensearch.yml"]
				if !exists {
					return false
				}
				return strings.Contains(data, "foo: bar\n") && strings.Contains(data, "bar: baz\n")
			}, timeout, interval).Should(BeTrue())

		})
	})

})
