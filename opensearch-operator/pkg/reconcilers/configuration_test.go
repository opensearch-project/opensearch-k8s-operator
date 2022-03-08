package reconcilers

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"

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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}}}

			reconcilerContext := NewReconcilerContext()

			underTest := NewConfigurationReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			configMap := corev1.ConfigMap{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-config", Namespace: clusterName}, &configMap)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When Reconciling the configuration controller with some configuration snippets", func() {
		It("should create a configmap ", func() {
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}}}

			reconcilerContext := NewReconcilerContext()

			underTest := NewConfigurationReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			reconcilerContext.AddConfig("foo", "bar")
			reconcilerContext.AddConfig("bar", "something")
			reconcilerContext.AddConfig("bar", "baz")
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			configMap := corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-config", Namespace: clusterName}, &configMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			data, exists := configMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			Expect(strings.Contains(data, "foo: bar\n")).To(BeTrue())
			Expect(strings.Contains(data, "bar: baz\n")).To(BeTrue())
		})
	})

	Context("When Reconciling the configuration controller with extra config", func() {
		clusterName := "extra-config"
		It("should create a configmap with these items", func() {
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{
					ExtraConfig: "foo: bar\nother.item.to.add: true",
				}}}

			reconcilerContext := NewReconcilerContext()

			underTest := NewConfigurationReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			configMap := corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-config", Namespace: clusterName}, &configMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			data, exists := configMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			Expect(strings.Contains(data, "foo: bar\n")).To(BeTrue())
			Expect(strings.Contains(data, "other.item.to.add: true\n")).To(BeTrue())
		})
	})

})
