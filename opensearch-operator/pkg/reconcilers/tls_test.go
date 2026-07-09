package reconcilers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	pkitls "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// makeTestCertPEM creates a self-signed certificate with the given validity window
func makeTestCertPEM(notBefore time.Time, notAfter time.Time) []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).ToNot(HaveOccurred())
	buffer := new(bytes.Buffer)
	Expect(pem.Encode(buffer, &pem.Block{Type: "CERTIFICATE", Bytes: der})).To(Succeed())
	return buffer.Bytes()
}

func newTLSReconciler(k8sClient *k8s.MockK8sClient, spec *opensearchv1.OpenSearchCluster) (*ReconcilerContext, *TLSReconciler) {
	reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, spec.Spec.NodePools)
	underTest := &TLSReconciler{
		client:            k8sClient,
		reconcilerContext: &reconcilerContext,
		instance:          spec,
		logger:            log.FromContext(context.Background()),
		pki:               helpers.NewMockPKI(),
	}
	underTest.pki = helpers.NewMockPKI()
	return &reconcilerContext, underTest
}

var _ = Describe("TLS Controller", func() {

	Context("When Reconciling the TLS configuration with no existing secrets", func() {
		It("should create the needed secrets ", func() {
			clusterName := "tls-test"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true},
						Http:      &opensearchv1.TlsConfigHttp{Generate: true},
					}},
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(transportSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(httpSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(adminSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == adminSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == transportSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-test,OU=tls-test\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin,OU=tls-test\"]"))
		})
	})

	Context("When Reconciling the TLS configuration with no existing secrets and perNode certs activated", func() {
		It("should create the needed secrets ", func() {
			clusterName := "tls-pernode"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true, PerNode: true},
						Http:      &opensearchv1.TlsConfigHttp{Generate: true},
					}},
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Replicas:  3,
						},
						{
							// sufficiently large to be above the pool cap
							Component: "data",
							Replicas:  12,
						},
					},
				}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(transportSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(httpSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(adminSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == adminSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool {
				if secret.ObjectMeta.Name != transportSecretName {
					return false
				}
				if _, exists := secret.Data["ca.crt"]; !exists {
					fmt.Printf("ca.crt missing from transport secret\n")
					return false
				}
				for _, nodePool := range spec.Spec.NodePools {
					var i int32
					for i = 0; i < nodePool.Replicas; i++ {
						name := fmt.Sprintf("tls-pernode-%s-%d", nodePool.Component, i)
						if _, exists := secret.Data[name+".crt"]; !exists {
							fmt.Printf("%s.crt missing from transport secret\n", name)
							return false
						}
						if _, exists := secret.Data[name+".key"]; !exists {
							fmt.Printf("%s.key missing from transport secret\n", name)
							return false
						}
					}
				}
				return true
			},
			)).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
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
		})
	})

	Context("When Reconciling the TLS configuration with external certificates", func() {
		It("Should not create secrets but only mount them", func() {
			clusterName := "tls-test-existingsecrets"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{General: opensearchv1.GeneralConfig{Version: "2.8.0"}, Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
					Transport: &opensearchv1.TlsConfigTransport{
						Generate: false,
						TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
							Secret:   corev1.LocalObjectReference{Name: "cert-transport"},
							CaSecret: corev1.LocalObjectReference{Name: "casecret-transport"},
						},
						NodesDn: []string{"CN=mycn", "CN=othercn"},
					},
					Http: &opensearchv1.TlsConfigHttp{
						Generate: false,
						TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
							Secret:   corev1.LocalObjectReference{Name: "cert-http"},
							CaSecret: corev1.LocalObjectReference{Name: "casecret-http"},
						},
						AdminDn: []string{"CN=admin1", "CN=admin2"},
					},
				},
				}}}
			data := map[string][]byte{
				"ca.crt": []byte("ca.crt"),
				"ca.key": []byte("ca.key"),
			}
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "casecret-http", Namespace: clusterName},
				Data:       data,
			}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret("casecret-http", clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(4))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(4))
			// With new mounting logic: CaSecret.Name != Secret.Name, so we mount both as directories
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "casecret-transport", "transport-ca")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "cert-transport", "transport-certs")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "casecret-http", "http-ca")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "cert-http", "http-certs")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin,OU=" + clusterName + "\"]"))
		})
	})

	Context("When Reconciling the TLS configuration with external per-node certificates", func() {
		It("Should not create secrets but only mount them", func() {
			clusterName := "tls-test-existingsecretspernode"

			caSecretName := clusterName + "-ca"
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: clusterName},
				Data: map[string][]byte{
					"ca.crt": []byte("ca.crt"),
					"ca.key": []byte("ca.key"),
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{General: opensearchv1.GeneralConfig{}, Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
					Transport: &opensearchv1.TlsConfigTransport{
						Generate: false,
						PerNode:  true,
						TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
							Secret: corev1.LocalObjectReference{Name: "my-transport-certs"},
						},
						NodesDn: []string{"CN=mycn", "CN=othercn"},
					},
					Http: &opensearchv1.TlsConfigHttp{
						Generate: false,
						TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
							Secret: corev1.LocalObjectReference{Name: "my-http-certs"},
						},
					},
				},
				}}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
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
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{General: opensearchv1.GeneralConfig{Version: "2.8.0"}, Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
					Transport: &opensearchv1.TlsConfigTransport{
						Generate: true,
						PerNode:  true,
						TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
							CaSecret: corev1.LocalObjectReference{Name: caSecretName},
						},
					},
					Http: &opensearchv1.TlsConfigHttp{
						Generate: true,
						TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
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

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-transport-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-http-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-transport-cert" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-http-cert" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-transport-cert", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-http-cert", "http-cert")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-withca-*,OU=tls-withca\"]"))

			// Verify that the CA cert path uses tls-http/ (not tls-http-ca/) since generate=true
			// includes the CA cert in the generated secret (Fixes #1279)
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.ssl.http.pemtrustedcas_filepath"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("tls-http/ca.crt"))
		})
	})

	Context("When Reconciling the TLS configuration with same CaSecret and Secret names", func() {
		It("Should mount only one secret as directory", func() {
			clusterName := "tls-same-secrets"
			caSecretName := "same-secret"
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: clusterName},
				Data: map[string][]byte{
					"ca.crt": []byte("ca.crt"),
					"ca.key": []byte("ca.key"),
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{Version: "2.8.0"},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{
							Generate: false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								Secret:   corev1.LocalObjectReference{Name: "same-secret"},
								CaSecret: corev1.LocalObjectReference{Name: "same-secret"}, // Same name
							},
							NodesDn: []string{"CN=mycn"},
						},
						Http: &opensearchv1.TlsConfigHttp{
							Generate: false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								Secret:   corev1.LocalObjectReference{Name: "same-secret"},
								CaSecret: corev1.LocalObjectReference{Name: caSecretName}, // Same name
							},
						},
					},
					},
				}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// Should have only 2 volumes/mounts (one for transport, one for http)
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "same-secret", "transport-certs")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, "same-secret", "http-certs")).Should((BeTrue()))
		})
	})

	Context("When Reconciling the TLS configuration with hot reload enabled", func() {
		It("Should enable hot reload configuration for supported versions", func() {
			clusterName := "tls-hotreload"
			caSecretName := "casecret-http"
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: clusterName},
				Data: map[string][]byte{
					"ca.crt": []byte("ca.crt"),
					"ca.key": []byte("ca.key"),
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{Version: "2.19.1"}, // Version that supports hot reload
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{
							Generate: false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								Secret:          corev1.LocalObjectReference{Name: "cert-transport"},
								CaSecret:        corev1.LocalObjectReference{Name: "casecret-transport"},
								EnableHotReload: ptr.To(true),
							},
							NodesDn: []string{"CN=mycn"},
						},
						Http: &opensearchv1.TlsConfigHttp{
							Generate: false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								Secret:          corev1.LocalObjectReference{Name: "cert-http"},
								CaSecret:        corev1.LocalObjectReference{Name: caSecretName},
								EnableHotReload: ptr.To(true),
							},
						},
					},
					},
				}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// Check that hot reload is enabled
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.ssl.certificates_hot_reload.enabled"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("true"))
		})

		It("Should not enable hot reload configuration for unsupported versions", func() {
			clusterName := "tls-hotreload-unsupported"
			caSecretName := "casecret-http"
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: clusterName},
				Data: map[string][]byte{
					"ca.crt": []byte("ca.crt"),
					"ca.key": []byte("ca.key"),
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{Version: "2.18.0"}, // Version that doesn't support hot reload
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{
							Generate: false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								Secret:          corev1.LocalObjectReference{Name: "cert-transport"},
								CaSecret:        corev1.LocalObjectReference{Name: "casecret-transport"},
								EnableHotReload: ptr.To(true),
							},
							NodesDn: []string{"CN=mycn"},
						},
						Http: &opensearchv1.TlsConfigHttp{
							Generate: false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								Secret:          corev1.LocalObjectReference{Name: "cert-http"},
								CaSecret:        corev1.LocalObjectReference{Name: caSecretName},
								EnableHotReload: ptr.To(true),
							},
						},
					},
					},
				}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// Check that hot reload is not enabled for unsupported version
			_, exists := reconcilerContext.OpenSearchConfig["plugins.security.ssl.certificates_hot_reload.enabled"]
			Expect(exists).To(BeFalse())
		})

		It("Should enable hot reload by default on OpenSearch 3.x", func() {
			clusterName := "tls-hotreload-default"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{Version: "3.0.0"},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true},
						Http:      &opensearchv1.TlsConfigHttp{Generate: true},
					}},
				}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background()).Maybe()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(mock.Anything, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.Anything).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.ssl.certificates_hot_reload.enabled"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("true"))
		})

		It("Should honor an explicit enableHotReload=false on OpenSearch 3.x", func() {
			clusterName := "tls-hotreload-optout"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{Version: "3.0.0"},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{
							Generate:             true,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{EnableHotReload: ptr.To(false)},
						},
						Http: &opensearchv1.TlsConfigHttp{
							Generate:             true,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{EnableHotReload: ptr.To(false)},
						},
					}},
				}}
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background()).Maybe()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(mock.Anything, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.Anything).Return(&ctrl.Result{}, nil)
			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			_, exists := reconcilerContext.OpenSearchConfig["plugins.security.ssl.certificates_hot_reload.enabled"]
			Expect(exists).To(BeFalse())
		})
	})

	Context("When Reconciling the TLS configuration with custom FQDN", func() {
		It("Should include custom FQDN in certificate DNS names", func() {
			clusterName := "tls-custom-fqdn"
			customFQDN := "opensearch.example.com"
			caSecretName := clusterName + "-ca"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						HttpPort:    9200,
					},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true},
						Http: &opensearchv1.TlsConfigHttp{
							Generate:   true,
							CustomFQDN: &customFQDN,
						},
					}},
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-transport-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(httpSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(adminSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == adminSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-transport-cert" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-transport-cert", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-http-cert", "http-cert")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-custom-fqdn,OU=tls-custom-fqdn\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin,OU=tls-custom-fqdn\"]"))
		})

		It("Should handle empty custom FQDN gracefully", func() {
			clusterName := "tls-empty-fqdn"
			emptyFQDN := ""
			caSecretName := clusterName + "-ca"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						HttpPort:    9200,
					},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true},
						Http: &opensearchv1.TlsConfigHttp{
							Generate:   true,
							CustomFQDN: &emptyFQDN,
						},
					}},
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-transport-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(httpSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(adminSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == adminSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-transport-cert" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-transport-cert", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(reconcilerContext.Volumes, reconcilerContext.VolumeMounts, clusterName+"-http-cert", "http-cert")).Should((BeTrue()))

			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-empty-fqdn,OU=tls-empty-fqdn\"]"))
			value, exists = reconcilerContext.OpenSearchConfig["plugins.security.authcz.admin_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=admin,OU=tls-empty-fqdn\"]"))
		})
	})

	Context("When deciding if a generated certificate should be renewed", func() {
		It("should renew expired or unparseable certificates and honor the rotation window", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "tls-renewal-decision", Namespace: "tls-renewal-decision", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true, RotateDaysBeforeExpiry: -1},
						Http:      &opensearchv1.TlsConfigHttp{Generate: true, RotateDaysBeforeExpiry: 30},
					}},
				},
			}
			_, underTest := newTLSReconciler(k8s.NewMockK8sClient(GinkgoT()), &spec)
			transportCd := certDescription{loggingName: "global", certContext: CertContextTransport}
			httpCd := certDescription{loggingName: "global", certContext: CertContextHttp}
			// An unparseable CA cannot be verified against, so it never forces a renewal
			mockCA := helpers.NewMockPKI().CAFromSecret(nil)

			expired := makeTestCertPEM(time.Now().AddDate(-1, 0, 0), time.Now().AddDate(0, 0, -1))
			valid10Days := makeTestCertPEM(time.Now().AddDate(-1, 0, 0), time.Now().AddDate(0, 0, 10))
			valid200Days := makeTestCertPEM(time.Now(), time.Now().AddDate(0, 0, 200))

			// Expired or broken certificates are replaced even with rotation disabled
			Expect(underTest.certShouldBeRenewed(mockCA, transportCd, expired)).To(BeTrue())
			Expect(underTest.certShouldBeRenewed(mockCA, transportCd, []byte("not a certificate"))).To(BeTrue())
			// Rotation disabled: a still-valid certificate is left alone
			Expect(underTest.certShouldBeRenewed(mockCA, transportCd, valid10Days)).To(BeFalse())
			// Rotation enabled: renewed only within the window
			Expect(underTest.certShouldBeRenewed(mockCA, httpCd, valid10Days)).To(BeTrue())
			Expect(underTest.certShouldBeRenewed(mockCA, httpCd, valid200Days)).To(BeFalse())
		})

		It("should renew certificates when the CA is replaced", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "tls-ca-replaced", Namespace: "tls-ca-replaced", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true, RotateDaysBeforeExpiry: 30},
						Http:      &opensearchv1.TlsConfigHttp{Generate: true, RotateDaysBeforeExpiry: 30},
					}},
				},
			}
			_, underTest := newTLSReconciler(k8s.NewMockK8sClient(GinkgoT()), &spec)
			transportCd := certDescription{loggingName: "global", certContext: CertContextTransport}

			pki := pkitls.NewPKI()
			originalCA, err := pki.GenerateCA("tls-ca-replaced")
			Expect(err).ToNot(HaveOccurred())
			// Same subject as the original, so an issuer name comparison cannot tell them apart
			replacedCA, err := pki.GenerateCA("tls-ca-replaced")
			Expect(err).ToNot(HaveOccurred())
			nodeCert, err := originalCA.CreateAndSignCertificate("node", "tls-ca-replaced", nil, 200*24*time.Hour)
			Expect(err).ToNot(HaveOccurred())

			Expect(underTest.certShouldBeRenewed(originalCA, transportCd, nodeCert.CertData())).To(BeFalse())
			Expect(underTest.certShouldBeRenewed(replacedCA, transportCd, nodeCert.CertData())).To(BeTrue())
		})
	})

	Context("When Reconciling the TLS configuration with existing generated certificates", func() {
		clusterName := "tls-renewal"

		newRenewalSpec := func(perNode bool) *opensearchv1.OpenSearchCluster {
			return &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					Security: &opensearchv1.Security{Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{Generate: true, PerNode: perNode, RotateDaysBeforeExpiry: 30},
						Http:      &opensearchv1.TlsConfigHttp{Generate: true, RotateDaysBeforeExpiry: 30},
					}},
					NodePools: []opensearchv1.NodePool{{Component: "masters", Replicas: 2}},
				},
				Status: opensearchv1.ClusterStatus{Initialized: true},
			}
		}

		setupMocks := func(transportSecret corev1.Secret, httpSecret corev1.Secret) (*k8s.MockK8sClient, func() *corev1.Secret, func() *corev1.Secret) {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background()).Maybe()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(clusterName+"-ca", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-transport-cert", clusterName).Return(transportSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-http-cert", clusterName).Return(httpSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-ca" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			var storedTransport, storedHttp *corev1.Secret
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-transport-cert" })).
				Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
					storedTransport = secret
					return &ctrl.Result{}, nil
				})
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-http-cert" })).
				Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
					storedHttp = secret
					return &ctrl.Result{}, nil
				})
			return mockClient, func() *corev1.Secret { return storedTransport }, func() *corev1.Secret { return storedHttp }
		}

		It("should renew an expired certificate and mark the secret for a rolling restart", func() {
			expiredCert := makeTestCertPEM(time.Now().AddDate(-1, 0, 0), time.Now().AddDate(0, 0, -1))
			validCert := makeTestCertPEM(time.Now(), time.Now().AddDate(0, 0, 200))
			transportSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-transport-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": expiredCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			httpSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-http-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			mockClient, storedTransport, storedHttp := setupMocks(transportSecret, httpSecret)

			reconcilerContext, underTest := newTLSReconciler(mockClient, newRenewalSpec(false))
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			marker := storedTransport().Annotations[CertRenewalAnnotation]
			Expect(marker).ToNot(BeEmpty())
			Expect(reconcilerContext.CertHashData).To(Equal([]string{"transport-certs:" + marker}))
			// The http certificate was still valid and must be left alone
			Expect(storedHttp().Annotations).ToNot(HaveKey(CertRenewalAnnotation))
			Expect(storedHttp().Data["tls.crt"]).To(Equal(validCert))
		})

		It("should not touch certificates outside the rotation window", func() {
			validCert := makeTestCertPEM(time.Now(), time.Now().AddDate(0, 0, 200))
			transportSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-transport-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			httpSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-http-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			mockClient, storedTransport, _ := setupMocks(transportSecret, httpSecret)

			reconcilerContext, underTest := newTLSReconciler(mockClient, newRenewalSpec(false))
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(storedTransport().Annotations).ToNot(HaveKey(CertRenewalAnnotation))
			Expect(storedTransport().Data["tls.crt"]).To(Equal(validCert))
			Expect(reconcilerContext.CertHashData).To(BeEmpty())
		})

		It("should keep the restart marker from a previous renewal", func() {
			validCert := makeTestCertPEM(time.Now(), time.Now().AddDate(0, 0, 200))
			transportSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        clusterName + "-transport-cert",
					Namespace:   clusterName,
					Annotations: map[string]string{CertRenewalAnnotation: "2027-01-01T00:00:00Z"},
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			httpSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-http-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			mockClient, _, _ := setupMocks(transportSecret, httpSecret)

			reconcilerContext, underTest := newTLSReconciler(mockClient, newRenewalSpec(false))
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(reconcilerContext.CertHashData).To(Equal([]string{"transport-certs:2027-01-01T00:00:00Z"}))
		})

		It("should renew expired per-node certificates and mark the secret for a rolling restart", func() {
			expiredCert := makeTestCertPEM(time.Now().AddDate(-1, 0, 0), time.Now().AddDate(0, 0, -1))
			validCert := makeTestCertPEM(time.Now(), time.Now().AddDate(0, 0, 200))
			transportSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-transport-cert", Namespace: clusterName},
				Data: map[string][]byte{
					"ca.crt":                       []byte("ca.crt"),
					clusterName + "-masters-0.crt": expiredCert,
					clusterName + "-masters-0.key": []byte("key"),
					clusterName + "-masters-1.crt": expiredCert,
					clusterName + "-masters-1.key": []byte("key"),
				},
			}
			httpSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-http-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			mockClient, storedTransport, _ := setupMocks(transportSecret, httpSecret)

			reconcilerContext, underTest := newTLSReconciler(mockClient, newRenewalSpec(true))
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			marker := storedTransport().Annotations[CertRenewalAnnotation]
			Expect(marker).ToNot(BeEmpty())
			Expect(reconcilerContext.CertHashData).To(Equal([]string{"transport-certs:" + marker}))
			// Both node certificates were replaced
			Expect(storedTransport().Data[clusterName+"-masters-0.crt"]).ToNot(Equal(expiredCert))
			Expect(storedTransport().Data[clusterName+"-masters-1.crt"]).ToNot(Equal(expiredCert))
		})

		It("should not mark the secret for a restart when hot reload is active", func() {
			expiredCert := makeTestCertPEM(time.Now().AddDate(-1, 0, 0), time.Now().AddDate(0, 0, -1))
			validCert := makeTestCertPEM(time.Now(), time.Now().AddDate(0, 0, 200))
			transportSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-transport-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": expiredCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			httpSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-http-cert", Namespace: clusterName},
				Type:       corev1.SecretTypeTLS,
				Data:       map[string][]byte{"tls.crt": validCert, "tls.key": []byte("key"), "ca.crt": []byte("ca.crt")},
			}
			mockClient, storedTransport, _ := setupMocks(transportSecret, httpSecret)

			spec := newRenewalSpec(false)
			// Hot reload is on by default for OpenSearch 3.x
			spec.Spec.General.Version = "3.0.0"
			reconcilerContext, underTest := newTLSReconciler(mockClient, spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// The certificate was renewed, but nodes reload it in place
			Expect(storedTransport().Annotations[CertRenewalAnnotation]).ToNot(BeEmpty())
			Expect(reconcilerContext.CertHashData).To(BeEmpty())
			Expect(reconcilerContext.OpenSearchConfig["plugins.security.ssl.certificates_hot_reload.enabled"]).To(Equal("true"))
		})
	})
})
