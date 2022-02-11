package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TLS Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName         = "tls-test"
		caSecretName        = "tls-test-ca"
		transportSecretName = "tls-test-transport-cert"
		httpSecretName      = "tls-test-http-cert"
		timeout             = time.Second * 30
		interval            = time.Second * 1
	)

	spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: clusterName}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{Transport: &opsterv1.TlsInterfaceConfig{Generate: true}, Http: &opsterv1.TlsInterfaceConfig{Generate: true}}}}}

	Context("When Reconciling the TLS configuration with no existing secrets", func() {
		It("should create the needed secrets ", func() {

			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(controllerContext.Volumes).Should(HaveLen(2))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(2))

			Eventually(func() bool {
				caSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &caSecret)
				if err != nil {
					return false
				}
				transportSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: transportSecretName, Namespace: clusterName}, &transportSecret)
				if err != nil {
					return false
				}
				httpSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: httpSecretName, Namespace: clusterName}, &httpSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

})
