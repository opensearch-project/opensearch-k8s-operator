package reconcilers

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Securityconfig Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "securityconfig"
		timeout     = time.Second * 10
		interval    = time.Second * 1
	)

	Context("When Reconciling the securityconfig reconciler with no securityconfig provided", func() {
		It("should not do anything ", func() {
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}}}

			reconcilerContext := NewReconcilerContext()
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			result, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())

		})
	})

	Context("When Reconciling the securityconfig reconciler with securityconfig secret configured but not available", func() {
		It("should trigger a requeue", func() {
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "foobar"},
							AdminSecret:          corev1.LocalObjectReference{Name: "admin"},
						},
					},
				}}

			reconcilerContext := NewReconcilerContext()
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			result, err := underTest.Reconcile()
			Expect(err).To(HaveOccurred())
			Expect(result.IsZero()).To(BeFalse())
		})
	})

	Context("When Reconciling the securityconfig reconciler with securityconfig secret configured and available", func() {
		It("should start an update job", func() {
			// Create namespace and secrets first
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())
			configSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig", Namespace: clusterName},
				StringData: map[string]string{"config.yml": "foobar"},
			}
			err = k8sClient.Create(context.Background(), &configSecret)
			Expect(err).ToNot(HaveOccurred())
			adminCertSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "admin-cert", Namespace: clusterName},
				StringData: map[string]string{"tls.crt": "foobar"},
			}
			err = k8sClient.Create(context.Background(), &adminCertSecret)
			Expect(err).ToNot(HaveOccurred())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig"},
							AdminSecret:          corev1.LocalObjectReference{Name: "admin-cert"},
						},
					},
				}}

			reconcilerContext := NewReconcilerContext()
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			job := batchv1.Job{}
			Eventually(func() bool {

				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-securityconfig-update", Namespace: clusterName}, &job)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})
})
