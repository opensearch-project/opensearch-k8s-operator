package reconcilers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	pkitls "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/kralicky/kmatch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	//+kubebuilder:scaffold:imports
)

func newDashboardsReconciler(k8sClient *k8s.MockK8sClient, spec *opensearchv1.OpenSearchCluster) (ReconcilerContext, *DashboardsReconciler) {
	reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, spec.Spec.NodePools)
	underTest := &DashboardsReconciler{
		client:            k8sClient,
		reconcilerContext: &reconcilerContext,
		recorder:          &helpers.MockEventRecorder{},
		instance:          spec,
		logger:            log.FromContext(context.Background()),
		pki:               helpers.NewMockPKI(),
	}
	underTest.pki = helpers.NewMockPKI()
	return reconcilerContext, underTest
}

// setupDashboardsCredentialsSecretMocks sets up mocks for dashboards credentials secret creation
func setupDashboardsCredentialsSecretMocks(mockClient *k8s.MockK8sClient, clusterName string) {
	dashboardsSecretName := clusterName + "-dashboards-password"
	mockClient.On("GetSecret", dashboardsSecretName, clusterName).Return(corev1.Secret{}, NotFoundError()).Once()
	mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool {
		return secret.Name == dashboardsSecretName
	})).Return(&ctrl.Result{}, nil)
	mockClient.On("GetSecret", dashboardsSecretName, clusterName).Return(corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: dashboardsSecretName, Namespace: clusterName},
		Data: map[string][]byte{
			"username": []byte("kibanaserver"),
			"password": []byte("test-password"),
		},
	}, nil).Once()
	mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).Return(&ctrl.Result{}, nil)
}

// signTestCert issues a certificate from the CA that expires at notAfter
func signTestCertExpiringAt(ca pkitls.Cert, notAfter time.Time) []byte {
	caCertBlock, _ := pem.Decode(ca.CertData())
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	Expect(err).ToNot(HaveOccurred())
	caKeyBlock, _ := pem.Decode(ca.KeyData())
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	Expect(err).ToNot(HaveOccurred())
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "dashboards"},
		NotBefore:    time.Now().AddDate(-1, 0, 0),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	Expect(err).ToNot(HaveOccurred())
	buffer := new(bytes.Buffer)
	Expect(pem.Encode(buffer, &pem.Block{Type: "CERTIFICATE", Bytes: der})).To(Succeed())
	return buffer.Bytes()
}

// reconcileDashboardsWithCert runs the reconciler against an existing dashboards cert Secret holding certData.
// It returns the Secret written back (nil if untouched) and the Deployment created.
func reconcileDashboardsWithCert(clusterName string, ca pkitls.Cert, certData []byte, rotateDays int) (*corev1.Secret, *appsv1.Deployment) {
	mockClient := k8s.NewMockK8sClient(GinkgoT())
	spec := opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{ServiceName: clusterName},
			Dashboards: opensearchv1.DashboardsConfig{
				Enable: true,
				Tls: &opensearchv1.DashboardsTlsConfig{
					Enable:                 true,
					Generate:               true,
					RotateDaysBeforeExpiry: rotateDays,
				},
				Service: opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
			},
		}}
	certSecretName := clusterName + "-dashboards-cert"
	mockClient.EXPECT().Scheme().Return(scheme.Scheme)
	mockClient.EXPECT().Context().Return(context.Background())
	mockClient.EXPECT().GetSecret(clusterName+"-ca", clusterName).Return(corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-ca", Namespace: clusterName},
		Data:       ca.SecretDataCA(),
	}, nil)
	mockClient.EXPECT().GetSecret(certSecretName, clusterName).Return(corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: certSecretName, Namespace: clusterName},
		Data:       map[string][]byte{"tls.crt": certData, "tls.key": []byte("old-key"), "ca.crt": ca.CertData()},
	}, nil)
	setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
	mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
	mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)
	var createdDeployment *appsv1.Deployment
	mockClient.On("CreateDeployment", mock.Anything).
		Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
			createdDeployment = deployment
			return &ctrl.Result{}, nil
		})
	var updatedSecret *corev1.Secret
	mockClient.On("UpdateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool {
		return secret.Name == certSecretName
	})).
		Return(func(secret *corev1.Secret) error {
			updatedSecret = secret
			return nil
		}).Maybe()

	_, underTest := newDashboardsReconciler(mockClient, &spec)
	underTest.pki = pkitls.NewPKI()
	_, err := underTest.Reconcile()
	Expect(err).ToNot(HaveOccurred())
	Expect(createdDeployment).ToNot(BeNil())
	return updatedSecret, createdDeployment
}

var _ = Describe("Dashboards Reconciler", func() {

	When("running the dashboards reconciler with TLS enabled and an existing cert in a single secret", func() {
		It("should mount the secret", func() {
			clusterName := "dashboards-singlesecret"
			secretName := "my-cert"
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						Tls: &opensearchv1.DashboardsTlsConfig{
							Enable:               true,
							Generate:             false,
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{Secret: corev1.LocalObjectReference{Name: secretName}},
						},
						Service: opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
					},
				},
			}

			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)

			_, underTest := newDashboardsReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(createdDeployment).ToNot(BeNil())
			Expect(helpers.CheckVolumeExists(createdDeployment.Spec.Template.Spec.Volumes, createdDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, secretName, "tls-cert")).Should((BeTrue()))
		})
	})

	When("running the dashboards reconciler with TLS enabled and generate enabled", func() {
		It("should create a cert", func() {
			clusterName := "dashboards-test-generate"
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						Tls: &opensearchv1.DashboardsTlsConfig{
							Enable:   true,
							Generate: true,
						},
						Service: opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().GetSecret(clusterName+"-ca", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-dashboards-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})
			var createdSecret *corev1.Secret
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool {
				return secret.Name == clusterName+"-dashboards-cert" || secret.Name == clusterName+"-ca"
			})).
				Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
					if secret.Name == clusterName+"-dashboards-cert" {
						createdSecret = secret
					}
					return &ctrl.Result{}, nil
				})
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)

			_, underTest := newDashboardsReconciler(mockClient, &spec)
			underTest.pki = helpers.NewMockPKI()
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// Check if secret is mounted
			Expect(helpers.CheckVolumeExists(createdDeployment.Spec.Template.Spec.Volumes, createdDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, clusterName+"-dashboards-cert", "tls-cert")).Should((BeTrue()))
			// Check if secret contains correct data keys
			Expect(helpers.HasKeyWithBytes(createdSecret.Data, "tls.key")).To(BeTrue())
			Expect(helpers.HasKeyWithBytes(createdSecret.Data, "tls.crt")).To(BeTrue())
		})
	})

	When("running the dashboards reconciler with a credentials secret supplied", func() {
		It("should provide these credentials as env vars", func() {
			clusterName := "dashboards-creds"
			credentialsSecret := clusterName + "-creds"
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable:                      true,
						OpensearchCredentialsSecret: corev1.LocalObjectReference{Name: credentialsSecret},
						Service:                     opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})

			_, underTest := newDashboardsReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(createdDeployment).To(
				HaveMatchingContainer(
					HaveEnv(
						"OPENSEARCH_USERNAME",
						corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: credentialsSecret,
							},
							Key: "username",
						},
						"OPENSEARCH_PASSWORD",
						corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: credentialsSecret,
							},
							Key: "password",
						},
					),
				),
			)
		})
	})

	When("running the dashboards reconciler with additionalConfig supplied", func() {
		It("should populate the dashboard config with these values", func() {
			clusterName := "dashboards-add-config"
			testConfig := "some-config-here"
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						AdditionalConfig: map[string]string{
							"some-key": testConfig,
						},
						Service: opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})
			var createdCm *corev1.ConfigMap
			mockClient.On("CreateConfigMap", mock.Anything).
				Return(func(cm *corev1.ConfigMap) (*ctrl.Result, error) {
					createdCm = cm
					return &ctrl.Result{}, nil
				})

			_, underTest := newDashboardsReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(createdCm).ToNot(BeNil())
			data, exists := createdCm.Data[helpers.DashboardConfigName]
			Expect(exists).To(BeTrue())
			Expect(strings.Contains(data, testConfig)).To(BeTrue())

			expectedChecksum, _ := util.GetSha1Sum([]byte(data))
			Expect(createdDeployment.Spec.Template.ObjectMeta.Annotations[helpers.DashboardChecksumName]).To(Equal(expectedChecksum))
		})
	})

	When("running the dashboards reconciler with envs supplied", func() {
		It("should populate the dashboard env vars", func() {
			clusterName := "dashboards-add-env"
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						Env: []corev1.EnvVar{
							{
								Name:  "TEST",
								Value: "TEST",
							},
						},
						Service: opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
					},
				}}

			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})

			_, underTest := newDashboardsReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(createdDeployment).To(
				HaveMatchingContainer(
					HaveEnv(
						"TEST",
						"TEST",
					),
				),
			)
		})
	})

	When("running the dashboards reconciler with optional image spec supplied", func() {
		It("should populate the dashboard image specification with these values", func() {
			clusterName := "dashboards-add-image-spec"
			image := "docker.io/my-opensearch-dashboards:custom"
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			imagePullPolicy := corev1.PullAlways
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						ImageSpec: &opensearchv1.ImageSpec{
							Image:           &image,
							ImagePullPolicy: &imagePullPolicy,
						},
						Service: opensearchv1.DashboardsServiceSpec{Labels: map[string]string{}},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})

			_, underTest := newDashboardsReconciler(mockClient, &spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			actualImage := createdDeployment.Spec.Template.Spec.Containers[0].Image
			actualImagePullPolicy := createdDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy
			Expect(actualImage).To(Equal(image))
			Expect(actualImagePullPolicy).To(Equal(imagePullPolicy))
		})
	})

	When("running the dashboards reconciler with extra volumes", func() {
		It("should mount the volumes in the deployment", func() {
			clusterName := "dashboards-add-volumes"
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						AdditionalVolumes: []opensearchv1.AdditionalVolume{
							{
								Name: "test-secret",
								Path: "/opt/test-secret",
								Secret: &corev1.SecretVolumeSource{
									SecretName: "test-secret",
								},
							},
							{
								Name: "test-cm",
								Path: "/opt/test-cm",
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "test-cm",
									},
								},
							},
						},
					},
				}}

			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
			mockClient.EXPECT().CreateConfigMap(mock.Anything).Return(&ctrl.Result{}, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})
			_, underTest := newDashboardsReconciler(mockClient, spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(createdDeployment).To(
				And(HaveMatchingContainer(
					HaveVolumeMounts(
						"test-secret",
						"test-cm",
					),
				),
					HaveMatchingVolume(And(
						HaveName("test-secret"),
						HaveVolumeSource("Secret"),
					)),
					HaveMatchingVolume(And(
						HaveName("test-cm"),
						HaveVolumeSource("ConfigMap"),
					)),
				))
		})
	})
})

var _ = Describe("Dashboards Reconciler generated certificate renewal", func() {
	var ca, otherCa pkitls.Cert

	BeforeEach(func() {
		var err error
		ca, err = pkitls.NewPKI().GenerateCA("dashboards-renewal")
		Expect(err).ToNot(HaveOccurred())
		otherCa, err = pkitls.NewPKI().GenerateCA("dashboards-renewal-other")
		Expect(err).ToNot(HaveOccurred())
	})

	checksumOf := func(certData []byte) string {
		sum, err := util.GetSha1Sum(certData)
		Expect(err).ToNot(HaveOccurred())
		return sum
	}

	It("should rewrite an expired certificate", func() {
		expired := signTestCertExpiringAt(ca, time.Now().AddDate(0, 0, -1))
		secret, _ := reconcileDashboardsWithCert("dashboards-renew-expired", ca, expired, 0)
		Expect(secret).ToNot(BeNil())
		Expect(secret.Data["tls.crt"]).ToNot(Equal(expired))
		Expect(secret.Data["tls.key"]).ToNot(Equal([]byte("old-key")))
	})

	It("should rewrite an unparseable certificate", func() {
		secret, _ := reconcileDashboardsWithCert("dashboards-renew-garbage", ca, []byte("not a certificate"), 0)
		Expect(secret).ToNot(BeNil())
		Expect(secret.Data["tls.crt"]).ToNot(Equal([]byte("not a certificate")))
	})

	It("should rewrite a certificate that is not signed by the current CA", func() {
		foreign := signTestCertExpiringAt(otherCa, time.Now().AddDate(0, 0, 200))
		secret, _ := reconcileDashboardsWithCert("dashboards-renew-wrongca", ca, foreign, 0)
		Expect(secret).ToNot(BeNil())
		Expect(secret.Data["tls.crt"]).ToNot(Equal(foreign))
		Expect(secret.Data["ca.crt"]).To(Equal(ca.CertData()))
	})

	It("should renew a certificate inside the rotation window", func() {
		soon := signTestCertExpiringAt(ca, time.Now().AddDate(0, 0, 10))
		secret, _ := reconcileDashboardsWithCert("dashboards-renew-window", ca, soon, 30)
		Expect(secret).ToNot(BeNil())
		Expect(secret.Data["tls.crt"]).ToNot(Equal(soon))
	})

	It("should leave a certificate outside the rotation window alone", func() {
		healthy := signTestCertExpiringAt(ca, time.Now().AddDate(0, 0, 200))
		secret, _ := reconcileDashboardsWithCert("dashboards-renew-healthy", ca, healthy, 30)
		Expect(secret).To(BeNil())
	})

	It("should not rotate early when rotateDaysBeforeExpiry is unset", func() {
		soon := signTestCertExpiringAt(ca, time.Now().AddDate(0, 0, 10))
		secret, _ := reconcileDashboardsWithCert("dashboards-renew-disabled", ca, soon, 0)
		Expect(secret).To(BeNil())
	})

	It("should change the pod template checksum when the certificate is renewed", func() {
		expired := signTestCertExpiringAt(ca, time.Now().AddDate(0, 0, -1))
		secret, renewedDeployment := reconcileDashboardsWithCert("dashboards-renew-checksum", ca, expired, 0)
		Expect(secret).ToNot(BeNil())
		renewedChecksum := renewedDeployment.Spec.Template.Annotations[helpers.DashboardTlsChecksumName]
		Expect(renewedChecksum).To(Equal(checksumOf(secret.Data["tls.crt"])))
		Expect(renewedChecksum).ToNot(Equal(checksumOf(expired)))

		healthy := signTestCertExpiringAt(ca, time.Now().AddDate(0, 0, 200))
		_, healthyDeployment := reconcileDashboardsWithCert("dashboards-renew-checksum-ok", ca, healthy, 0)
		Expect(healthyDeployment.Spec.Template.Annotations[helpers.DashboardTlsChecksumName]).To(Equal(checksumOf(healthy)))
	})
})
