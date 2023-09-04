package reconcilers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newTLSReconciler(spec *opsterv1.OpenSearchCluster) (*ReconcilerContext, *TLSReconciler) {
	reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, spec.Spec.NodePools)
	underTest := NewTLSReconciler(
		k8sClient,
		context.Background(),
		&reconcilerContext,
		spec,
	)
	underTest.pki = helpers.NewMockPKI()
	return &reconcilerContext, underTest
}

var _ = Describe("TLS Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	Context("When Reconciling the TLS configuration with no existing secrets", func() {
		It("should create the needed secrets ", func() {
			clusterName := "tls-test"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						Transport: &opsterv1.TlsConfigTransport{Generate: true},
						Http:      &opsterv1.TlsConfigHttp{Generate: true},
					}},
				}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			//fmt.Printf("%s\n", reconcilerContext.OpenSearchConfig)
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-test,OU=tls-test\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin,OU=tls-test\"]"))

			Eventually(func() bool {
				caSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &caSecret)
				if err != nil {
					return false
				}
				adminSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: adminSecretName, Namespace: clusterName}, &adminSecret)
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

	Context("When Reconciling the TLS configuration with no existing secrets and perNode certs activated", func() {
		It("should create the needed secrets ", func() {
			clusterName := "tls-pernode"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						Transport: &opsterv1.TlsConfigTransport{Generate: true, PerNode: true},
						Http:      &opsterv1.TlsConfigHttp{Generate: true},
					}},
					NodePools: []opsterv1.NodePool{
						{
							Component: "masters",
							Replicas:  3,
						},
					},
				}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-pernode-*,OU=tls-pernode\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin,OU=tls-pernode\"]"))

			Eventually(func() bool {
				caSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &caSecret)
				if err != nil {
					return false
				}
				adminSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: adminSecretName, Namespace: clusterName}, &adminSecret)
				if err != nil {
					return false
				}
				transportSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: transportSecretName, Namespace: clusterName}, &transportSecret)
				if err != nil {
					return false
				}
				if _, exists := transportSecret.Data["ca.crt"]; !exists {
					fmt.Printf("ca.crt missing from transport secret\n")
					return false
				}
				for i := 0; i < 3; i++ {
					name := fmt.Sprintf("tls-pernode-masters-%d", i)
					if _, exists := transportSecret.Data[name+".crt"]; !exists {
						fmt.Printf("%s.crt missing from transport secret\n", name)
						return false
					}
					if _, exists := transportSecret.Data[name+".key"]; !exists {
						fmt.Printf("%s.key missing from transport secret\n", name)
						return false
					}
				}
				httpSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: httpSecretName, Namespace: clusterName}, &httpSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("When Reconciling the TLS configuration with external certificates", func() {
		It("Should not create secrets but only mount them", func() {
			clusterName := "tls-test-existingsecrets"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
					Transport: &opsterv1.TlsConfigTransport{
						Generate: false,
						TlsCertificateConfig: opsterv1.TlsCertificateConfig{
							Secret:   corev1.LocalObjectReference{Name: "cert-transport"},
							CaSecret: corev1.LocalObjectReference{Name: "casecret-transport"},
						},
						NodesDn: []string{"CN=mycn", "CN=othercn"},
						AdminDn: []string{"CN=admin1", "CN=admin2"},
					},
					Http: &opsterv1.TlsConfigHttp{
						Generate: false,
						TlsCertificateConfig: opsterv1.TlsCertificateConfig{
							Secret:   corev1.LocalObjectReference{Name: "cert-http"},
							CaSecret: corev1.LocalObjectReference{Name: "casecret-http"},
						},
					},
				},
				}}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(6))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(6))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "casecret-transport", "transport-ca")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "cert-transport", "transport-key")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "cert-transport", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "casecret-http", "http-ca")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "cert-http", "http-key")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "cert-http", "http-cert")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin1\",\"CN=admin2\"]"))
		})
	})

	Context("When Reconciling the TLS configuration with external per-node certificates", func() {
		It("Should not create secrets but only mount them", func() {
			clusterName := "tls-test-existingsecretspernode"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
					Transport: &opsterv1.TlsConfigTransport{
						Generate: false,
						PerNode:  true,
						TlsCertificateConfig: opsterv1.TlsCertificateConfig{
							Secret: corev1.LocalObjectReference{Name: "my-transport-certs"},
						},
						NodesDn: []string{"CN=mycn", "CN=othercn"},
					},
					Http: &opsterv1.TlsConfigHttp{
						Generate: false,
						TlsCertificateConfig: opsterv1.TlsCertificateConfig{
							Secret: corev1.LocalObjectReference{Name: "my-http-certs"},
						},
					},
				},
				}}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "my-transport-certs", "transport-certs")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "my-http-certs", "http-certs")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
		})
	})

	Context("When Reconciling the TLS configuration with external CA certificate", func() {
		It("Should create certificates using that CA", func() {
			clusterName := "tls-withca"
			caSecretName := clusterName + "-myca"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
					Transport: &opsterv1.TlsConfigTransport{
						Generate: true,
						PerNode:  true,
						TlsCertificateConfig: opsterv1.TlsCertificateConfig{
							CaSecret: corev1.LocalObjectReference{Name: caSecretName},
						},
					},
					Http: &opsterv1.TlsConfigHttp{
						Generate: true,
						TlsCertificateConfig: opsterv1.TlsCertificateConfig{
							CaSecret: corev1.LocalObjectReference{Name: caSecretName},
						},
					},
				},
				}}}
			data := map[string][]byte{
				"ca.crt": []byte("ca.crt"),
				"ca.key": []byte("ca.key"),
			}
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: clusterName},
				Data:       data,
			}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())

			err := k8sClient.Create(context.Background(), &caSecret)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				secret := corev1.Secret{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-transport-cert", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-http-cert", "http-cert")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-withca-*,OU=tls-withca\"]"))
		})
	})

})
