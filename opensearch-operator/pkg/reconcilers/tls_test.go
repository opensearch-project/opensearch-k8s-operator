package reconcilers

import (
	"context"
	"fmt"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
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

})
