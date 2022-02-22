package controllers

import (
	"context"
	"opensearch.opster.io/pkg/helpers"
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

	Context("When Reconciling the TLS configuration with no existing secrets", func() {
		It("should create the needed secrets ", func() {
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: clusterName}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{Transport: &opsterv1.TlsInterfaceConfig{Generate: true}, Http: &opsterv1.TlsInterfaceConfig{Generate: true}}}}}
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := helpers.K8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   helpers.K8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(controllerContext.Volumes).Should(HaveLen(2))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(2))
			//fmt.Printf("%s\n", controllerContext.OpenSearchConfig)
			value, exists := controllerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-test\"]"))

			Eventually(func() bool {
				caSecret := corev1.Secret{}
				err = helpers.K8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &caSecret)
				if err != nil {
					return false
				}
				transportSecret := corev1.Secret{}
				err = helpers.K8sClient.Get(context.Background(), client.ObjectKey{Name: transportSecretName, Namespace: clusterName}, &transportSecret)
				if err != nil {
					return false
				}
				httpSecret := corev1.Secret{}
				err = helpers.K8sClient.Get(context.Background(), client.ObjectKey{Name: httpSecretName, Namespace: clusterName}, &httpSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("When Reconciling the TLS configuration with external certificates", func() {
		It("Should not create secrets but only mount them", func() {
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: "tls-test-existingsecrets"}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
				Transport: &opsterv1.TlsInterfaceConfig{
					Generate:   false,
					CaSecret:   &opsterv1.TlsSecret{SecretName: "casecret-transport"},
					KeySecret:  &opsterv1.TlsSecret{SecretName: "keysecret-transport"},
					CertSecret: &opsterv1.TlsSecret{SecretName: "certsecret-transport"},
				},
				Http: &opsterv1.TlsInterfaceConfig{
					Generate:   false,
					CaSecret:   &opsterv1.TlsSecret{SecretName: "casecret-http"},
					KeySecret:  &opsterv1.TlsSecret{SecretName: "keysecret-http"},
					CertSecret: &opsterv1.TlsSecret{SecretName: "certsecret-http"},
				},
				NodesDn: []string{"CN=mycn", "CN=othercn"},
			},
			}}}
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tls-test-existingsecrets",
				},
			}
			err := helpers.K8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   helpers.K8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(controllerContext.Volumes).Should(HaveLen(6))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(6))
			Expect(checkVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "casecret-transport", "transport-ca")).Should((BeTrue()))
			Expect(checkVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "keysecret-transport", "transport-key")).Should((BeTrue()))
			Expect(checkVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "certsecret-transport", "transport-cert")).Should((BeTrue()))
			Expect(checkVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "casecret-http", "http-ca")).Should((BeTrue()))
			Expect(checkVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "keysecret-http", "http-key")).Should((BeTrue()))
			Expect(checkVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "certsecret-http", "http-cert")).Should((BeTrue()))

			value, exists := controllerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
		})
	})

})

func checkVolumeExists(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, secretName string, volumeName string) bool {
	for _, volume := range volumes {
		if volume.Name == volumeName {
			for _, mount := range volumeMounts {
				if mount.Name == volumeName {
					return volume.Secret.SecretName == secretName
				}
			}
			return false
		}
	}
	return false
}
