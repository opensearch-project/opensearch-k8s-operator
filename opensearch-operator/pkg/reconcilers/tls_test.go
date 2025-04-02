package reconcilers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func newTLSReconciler(k8sClient *k8s.MockK8sClient, spec *opsterv1.OpenSearchCluster) (*ReconcilerContext, *TLSReconciler) {
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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						Transport: &opsterv1.TlsConfigTransport{Generate: true},
						Http:      &opsterv1.TlsConfigHttp{Generate: true},
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

			// Mock for UpdateOpenSearchClusterStatus
			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool {
					return key.Name == clusterName && key.Namespace == clusterName
				}),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Return(nil)

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
				for i := 0; i < 3; i++ {
					name := fmt.Sprintf("tls-pernode-masters-%d", i)
					if _, exists := secret.Data[name+".crt"]; !exists {
						fmt.Printf("%s.crt missing from transport secret\n", name)
						return false
					}
					if _, exists := secret.Data[name+".key"]; !exists {
						fmt.Printf("%s.key missing from transport secret\n", name)
						return false
					}
				}
				return true
			},
			)).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)
			// Mock for UpdateOpenSearchClusterStatus
			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool {
					return key.Name == clusterName && key.Namespace == clusterName
				}),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Return(nil)

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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{Version: "2.8.0"}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
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
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
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
			mockClient := k8s.NewMockK8sClient(GinkgoT())

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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{Version: "2.8.0"}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
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

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(clusterName+"-transport-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-http-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-admin-cert", clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-transport-cert" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-http-cert" })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == clusterName+"-admin-cert" })).Return(&ctrl.Result{}, nil)
			// Mock for UpdateOpenSearchClusterStatus
			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool {
					return key.Name == clusterName && key.Namespace == clusterName
				}),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Return(nil)

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
		})
	})

	Context("When Reconciling the TLS configuration with ValidTill field", func() {
		It("should use the ValidTill field for certificate expiry", func() {
			clusterName := "tls-validtill"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"

			// Set ValidTill to 6 months from now
			validTill := "6M"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						ValidTill: validTill,
						Transport: &opsterv1.TlsConfigTransport{Generate: true},
						Http:      &opsterv1.TlsConfigHttp{Generate: true},
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

			// Capture the status update functions to verify certificate expiry fields
			var statusUpdateCallCount int
			var transportStatusUpdateFunc, httpStatusUpdateFunc func(*opsterv1.OpenSearchCluster)

			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool {
					return key.Name == clusterName && key.Namespace == clusterName
				}),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				statusUpdateCallCount++
				updateFunc := args.Get(1).(func(*opsterv1.OpenSearchCluster))

				// Create a test cluster to determine which field is being updated
				testCluster := &opsterv1.OpenSearchCluster{}
				updateFunc(testCluster)

				if !testCluster.Status.TransportCertificateExpiry.IsZero() {
					transportStatusUpdateFunc = updateFunc
				} else if !testCluster.Status.HttpCertificateExpiry.IsZero() {
					httpStatusUpdateFunc = updateFunc
				}
			}).Return(nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// Basic validation that the reconciler completed successfully
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-validtill,OU=tls-validtill\"]"))

			// Verify that the status fields were updated correctly
			if transportStatusUpdateFunc != nil {
				updatedCluster := &opsterv1.OpenSearchCluster{}
				transportStatusUpdateFunc(updatedCluster)
				Expect(updatedCluster.Status.TransportCertificateExpiry.IsZero()).To(BeFalse())
			}
			if httpStatusUpdateFunc != nil {
				updatedCluster := &opsterv1.OpenSearchCluster{}
				httpStatusUpdateFunc(updatedCluster)
				Expect(updatedCluster.Status.HttpCertificateExpiry.IsZero()).To(BeFalse())
			}
			Expect(statusUpdateCallCount).To(Equal(2))

		})
	})

	Context("When Reconciling the TLS configuration with invalid ValidTill field", func() {
		It("should error out", func() {
			clusterName := "tls-invalid-validtill"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"

			// Set an invalid ValidTill format
			invalidValidTill := "invalid-date-format"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						ValidTill: invalidValidTill,
						Transport: &opsterv1.TlsConfigTransport{Generate: true},
						Http:      &opsterv1.TlsConfigHttp{Generate: true},
					}},
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(transportSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)

			_, err := underTest.Reconcile()
			Expect(err).To(MatchError("invalid format, expected number followed by W, M, or Y"))

			// Basic validation that the reconciler completed successfully despite invalid date
			Expect(reconcilerContext.Volumes).Should(BeEmpty())
			Expect(reconcilerContext.VolumeMounts).Should(BeEmpty())
			_, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeFalse())

			mockCert := underTest.pki.(*helpers.PkiMock).GetUsedCertMock()
			Expect(mockCert.NumTimesCalledCreateAndSignCertificate).To(Equal(0))
			Expect(mockCert.NumTimesCalledCreateAndSignCertificateWithExpiry).To(Equal(0))
		})
	})

	Context("When Reconciling the TLS configuration with ValidTill field and perNode certs", func() {
		It("should use the ValidTill field for all node certificates", func() {
			clusterName := "tls-validtill-pernode"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			adminSecretName := clusterName + "-admin-cert"

			// Set ValidTill to 6 months from now
			// validTillDT := time.Now().UTC().AddDate(10, 0, 0)
			validTill := "10Y"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						ValidTill: validTill,
						Transport: &opsterv1.TlsConfigTransport{Generate: true, PerNode: true},
						Http:      &opsterv1.TlsConfigHttp{Generate: true},
					}},
					NodePools: []opsterv1.NodePool{
						{
							Component: "masters",
							Replicas:  2,
						},
					},
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
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool {
				if secret.ObjectMeta.Name != transportSecretName {
					return false
				}
				if _, exists := secret.Data["ca.crt"]; !exists {
					fmt.Printf("ca.crt missing from transport secret\n")
					return false
				}
				for i := 0; i < 2; i++ {
					name := fmt.Sprintf("tls-validtill-pernode-masters-%d", i)
					if _, exists := secret.Data[name+".crt"]; !exists {
						fmt.Printf("%s.crt missing from transport secret\n", name)
						return false
					}
					if _, exists := secret.Data[name+".key"]; !exists {
						fmt.Printf("%s.key missing from transport secret\n", name)
						return false
					}
				}
				return true
			})).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)

			// Capture the status update functions to verify certificate expiry fields
			var statusUpdateCallCount int
			var transportStatusUpdateFunc, httpStatusUpdateFunc func(*opsterv1.OpenSearchCluster)

			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool {
					return key.Name == clusterName && key.Namespace == clusterName
				}),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				statusUpdateCallCount++
				updateFunc := args.Get(1).(func(*opsterv1.OpenSearchCluster))

				// Create a test cluster to determine which field is being updated
				testCluster := &opsterv1.OpenSearchCluster{}
				updateFunc(testCluster)

				if !testCluster.Status.TransportCertificateExpiry.IsZero() {
					transportStatusUpdateFunc = updateFunc
				} else if !testCluster.Status.HttpCertificateExpiry.IsZero() {
					httpStatusUpdateFunc = updateFunc
				}
			}).Return(nil)

			// At the beginning of your test
			testRegistry := prometheus.NewRegistry()
			testRegistry.MustRegister(metrics.TLSCertExpiryDays)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			metricFamilies, err := testRegistry.Gather()
			Expect(err).ToNot(HaveOccurred())
			Expect(metricFamilies).To(HaveLen(1))
			// Expect(metricFamilies[0].GetMetric()).To(HaveLen(5))
			for _, metric := range metricFamilies[0].GetMetric() {
				Expect(metric.GetGauge().GetValue()).To(BeNumerically(">", 30.0))
			}

			mockCert := underTest.pki.(*helpers.PkiMock).GetUsedCertMock()
			Expect(mockCert.NumTimesCalledCreateAndSignCertificate).To(Equal(0))
			Expect(mockCert.NumTimesCalledCreateAndSignCertificateWithExpiry).To(Equal(5))

			// Basic validation that the reconciler completed successfully
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-validtill-pernode-*,OU=tls-validtill-pernode\"]"))
			Expect(transportStatusUpdateFunc).ToNot(BeNil())
			Expect(httpStatusUpdateFunc).ToNot(BeNil())
			// Verify that the status fields were updated correctly
			if transportStatusUpdateFunc != nil {
				updatedCluster := &opsterv1.OpenSearchCluster{}
				transportStatusUpdateFunc(updatedCluster)
				Expect(updatedCluster.Status.TransportCertificateExpiry.IsZero()).To(BeFalse())
			}
			if httpStatusUpdateFunc != nil {
				updatedCluster := &opsterv1.OpenSearchCluster{}
				httpStatusUpdateFunc(updatedCluster)
				Expect(updatedCluster.Status.HttpCertificateExpiry.IsZero()).To(BeFalse())
			}
			Expect(statusUpdateCallCount).To(Equal(2))
		})
	})

	Context("When Creating an OpenSearchCluster with only Transport TLS enabled", func() {
		It("should use the ValidTill field for all node certificates", func() {
			clusterName := "transport-only"
			caSecretName := clusterName + "-ca"
			transportSecretName := clusterName + "-transport-cert"
			httpSecretName := clusterName + "-http-cert"
			httpCASecretName := clusterName + "-http-ca"
			adminSecretName := clusterName + "-admin-cert"

			// Set ValidTill to 520 weeks from now
			validTillDT := time.Now().UTC().AddDate(0, 0, 520*7)
			validTill := "520W"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.0.0",
					},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						ValidTill: validTill,
						Transport: &opsterv1.TlsConfigTransport{
							Generate: true,
							PerNode:  true,
						},
						Http: &opsterv1.TlsConfigHttp{
							Generate: false, // HTTP TLS generation disabled
							TlsCertificateConfig: opsterv1.TlsCertificateConfig{
								Secret:   corev1.LocalObjectReference{Name: httpSecretName},
								CaSecret: corev1.LocalObjectReference{Name: httpCASecretName},
							},
						},
					}},
					NodePools: []opsterv1.NodePool{
						{
							Component:   "masters",
							Replicas:    1,
							Roles:       []string{"master", "data"},
							Persistence: &opsterv1.PersistenceConfig{PersistenceSource: opsterv1.PersistenceSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
					},
				},
			}

			data := map[string][]byte{
				"ca.crt": []byte("ca.crt"),
				"ca.key": []byte("ca.key"),
			}
			caSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: httpCASecretName, Namespace: clusterName},
				Data:       data,
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(transportSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(httpCASecretName, clusterName).Return(caSecret, nil)
			mockClient.EXPECT().GetSecret(adminSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == transportSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == adminSecretName })).Return(&ctrl.Result{}, nil)
			// Capture the status update function to verify certificate expiry fields
			var statusUpdateFunc func(*opsterv1.OpenSearchCluster)
			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool { return key.Name == clusterName && key.Namespace == clusterName }),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				statusUpdateFunc = args.Get(1).(func(*opsterv1.OpenSearchCluster))
			}).Return(nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			mockCert := underTest.pki.(*helpers.PkiMock).GetUsedCertMock()
			Expect(mockCert.NumTimesCalledCreateAndSignCertificate).To(Equal(0))
			Expect(mockCert.NumTimesCalledCreateAndSignCertificateWithExpiry).To(Equal(3))

			// Basic validation that the reconciler completed successfully
			Expect(reconcilerContext.Volumes).Should(HaveLen(4))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(4))
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=transport-only-*,OU=transport-only\"]"))
			Expect(statusUpdateFunc).ToNot(BeNil())

			// Verify that the status fields were updated correctly
			if statusUpdateFunc != nil {
				updatedCluster := &opsterv1.OpenSearchCluster{}
				statusUpdateFunc(updatedCluster)

				// http certificate expiry field should be set to the default expiry time
				Expect(updatedCluster.Status.TransportCertificateExpiry.IsZero()).To(BeFalse())
				Expect(updatedCluster.Status.HttpCertificateExpiry.IsZero()).To(BeTrue())

				// We are using testCert given in test-helpers.go, which is set to expire long time in the future
				transportExpiryDiff := updatedCluster.Status.TransportCertificateExpiry.Time.Sub(validTillDT)
				// expiry greater than 30 days
				Expect(transportExpiryDiff.Abs()).To(BeNumerically(">", 30.0))

			}
		})
	})

	Context("When Creating an OpenSearchCluster with only HTTP TLS enabled", func() {
		It("should use the ValidTill field for all node certificates", func() {
			clusterName := "http-only"
			caSecretName := clusterName + "-ca"
			httpSecretName := clusterName + "-http-cert"

			// Set ValidTill to 6 months from now
			validTillDT := time.Now().AddDate(0, 13, 0)
			validTill := "13M"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.0.0",
					},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						ValidTill: validTill,
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
							Generate: true, // HTTP TLS generation disabled
						},
					}},
					NodePools: []opsterv1.NodePool{
						{
							Component:   "masters",
							Replicas:    1,
							Roles:       []string{"master", "data"},
							Persistence: &opsterv1.PersistenceConfig{PersistenceSource: opsterv1.PersistenceSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
					},
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().GetSecret(caSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(httpSecretName, clusterName).Return(corev1.Secret{}, NotFoundError())

			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == httpSecretName })).Return(&ctrl.Result{}, nil)
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool { return secret.ObjectMeta.Name == caSecretName })).Return(&ctrl.Result{}, nil)
			// Capture the status update function to verify certificate expiry fields
			var statusUpdateFunc func(*opsterv1.OpenSearchCluster)
			mockClient.On("UpdateOpenSearchClusterStatus",
				mock.MatchedBy(func(key client.ObjectKey) bool { return key.Name == clusterName && key.Namespace == clusterName }),
				mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				statusUpdateFunc = args.Get(1).(func(*opsterv1.OpenSearchCluster))
			}).Return(nil)

			reconcilerContext, underTest := newTLSReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			mockCert := underTest.pki.(*helpers.PkiMock).GetUsedCertMock()
			Expect(mockCert.NumTimesCalledCreateAndSignCertificate).To(Equal(0))
			Expect(mockCert.NumTimesCalledCreateAndSignCertificateWithExpiry).To(Equal(1))

			// Basic validation that the reconciler completed successfully
			Expect(reconcilerContext.Volumes).Should(HaveLen(4))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(4))
			value, exists := reconcilerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
			Expect(statusUpdateFunc).ToNot(BeNil())

			// Verify that the status fields were updated correctly
			if statusUpdateFunc != nil {
				updatedCluster := &opsterv1.OpenSearchCluster{}
				statusUpdateFunc(updatedCluster)

				// transport certificate expiry field should be set to the default expiry time
				Expect(updatedCluster.Status.TransportCertificateExpiry.IsZero()).To(BeTrue())
				Expect(updatedCluster.Status.HttpCertificateExpiry.IsZero()).To(BeFalse())
				// We are using testCert given in test-helpers.go, which is set to expire long time in the future
				httpExpiryDiff := updatedCluster.Status.HttpCertificateExpiry.Time.Sub(validTillDT)
				Expect(httpExpiryDiff.Abs()).To(BeNumerically(">", 30.0))
			}
		})
	})

})

var _ = Describe("RFC3339 DateTime Generator", func() {
	Context("when input is valid", func() {
		It("should handle week durations correctly", func() {
			for _, weeks := range []string{"1W", "5W", "12W"} {
				result, err := GenerateRFC3339DateTime(weeks)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(Equal(time.Time{}))

				// Validate the duration calculation
				n, _ := strconv.Atoi(strings.TrimSuffix(weeks, "W"))
				expectedTime := time.Now().UTC().AddDate(0, 0, n*7)

				// Allow 1 second tolerance
				diff := expectedTime.Sub(result)
				if diff < 0 {
					diff = -diff
				}
				Expect(diff).To(BeNumerically("<", time.Second))
			}
		})

		It("should handle month durations correctly", func() {
			for _, months := range []string{"1M", "6M", "12M"} {
				result, err := GenerateRFC3339DateTime(months)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(Equal(time.Time{}))

				// Validate the duration calculation
				n, _ := strconv.Atoi(strings.TrimSuffix(months, "M"))
				expectedTime := time.Now().UTC().AddDate(0, n, 0)

				// Allow 1 second tolerance
				diff := expectedTime.Sub(result)
				if diff < 0 {
					diff = -diff
				}
				Expect(diff).To(BeNumerically("<", time.Second))
			}
		})

		It("should handle year durations correctly", func() {
			for _, years := range []string{"1Y", "5Y", "10Y"} {
				result, err := GenerateRFC3339DateTime(years)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(Equal(time.Time{}))

				// Validate the duration calculation
				n, _ := strconv.Atoi(strings.TrimSuffix(years, "Y"))
				expectedTime := time.Now().UTC().AddDate(n, 0, 0)

				// Allow 1 second tolerance
				diff := expectedTime.Sub(result)
				if diff < 0 {
					diff = -diff
				}
				Expect(diff).To(BeNumerically("<", time.Second))
			}
		})
	})

	Context("when input is invalid", func() {
		DescribeTable("should return error for invalid inputs",
			func(input string) {
				_, err := GenerateRFC3339DateTime(input)
				Expect(err).To(HaveOccurred())
			},
			Entry("empty string", ""),
			Entry("missing unit", "5"),
			Entry("lowercase unit", "5w"),
			Entry("invalid unit", "5D"),
			Entry("mixed units", "5W6M"),
			Entry("negative value", "-5W"),
			Entry("zero value", "0W"),
			Entry("non-numeric input", "abc"),
		)
	})
})
