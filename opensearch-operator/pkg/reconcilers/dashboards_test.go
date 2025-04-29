package reconcilers

import (
	"context"
	"strings"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
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

func newDashboardsReconciler(k8sClient *k8s.MockK8sClient, spec *opsterv1.OpenSearchCluster) (ReconcilerContext, *DashboardsReconciler) {
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

var _ = Describe("Dashboards Reconciler", func() {

	When("running the dashboards reconciler with TLS enabled and an existing cert in a single secret", func() {
		It("should mount the secret", func() {
			clusterName := "dashboards-singlesecret"
			secretName := "my-cert"
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Tls: &opsterv1.DashboardsTlsConfig{
							Enable:               true,
							Generate:             false,
							TlsCertificateConfig: opsterv1.TlsCertificateConfig{Secret: corev1.LocalObjectReference{Name: secretName}},
						},
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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Tls: &opsterv1.DashboardsTlsConfig{
							Enable:   true,
							Generate: true,
						},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().GetSecret(clusterName+"-ca", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-dashboards-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})
			var createdSecret *corev1.Secret
			mockClient.On("CreateSecret", mock.Anything).
				Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
					createdSecret = secret
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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable:                      true,
						OpensearchCredentialsSecret: corev1.LocalObjectReference{Name: credentialsSecret},
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

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						AdditionalConfig: map[string]string{
							"some-key": testConfig,
						},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().CreateService(mock.Anything).Return(&ctrl.Result{}, nil)
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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Env: []corev1.EnvVar{
							{
								Name:  "TEST",
								Value: "TEST",
							},
						},
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
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						ImageSpec: &opsterv1.ImageSpec{
							Image:           &image,
							ImagePullPolicy: &imagePullPolicy,
						},
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
			spec := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						AdditionalVolumes: []opsterv1.AdditionalVolume{
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
	When("running the dashboards reconciler with TLS enabled, generate enabled SAN supplied", func() {
		It("should create a cert", func() {
			clusterName := "dashboards-test-generate"
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			additionalSANs := []string{
				"opensearch.example.com",
				"custom-domain.example.org",
				"*.opensearch-domain.com",
			}
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Tls: &opsterv1.DashboardsTlsConfig{
							Enable:         true,
							Generate:       true,
							AdditionalSANs: additionalSANs,
						},
					},
				}}
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			mockClient.EXPECT().GetSecret(clusterName+"-ca", clusterName).Return(corev1.Secret{}, NotFoundError())
			mockClient.EXPECT().GetSecret(clusterName+"-dashboards-cert", clusterName).Return(corev1.Secret{}, NotFoundError())
			var createdDeployment *appsv1.Deployment
			mockClient.On("CreateDeployment", mock.Anything).
				Return(func(deployment *appsv1.Deployment) (*ctrl.Result, error) {
					createdDeployment = deployment
					return &ctrl.Result{}, nil
				})
			var createdSecret *corev1.Secret
			mockClient.On("CreateSecret", mock.Anything).
				Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
					createdSecret = secret
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

			mockCert := underTest.pki.(*helpers.PkiMock).GetUsedCertMock()

			// Check that the certificate was created with the expected DNS names
			Expect(mockCert.LastDnsNames).To(HaveLen(len(additionalSANs) + 4)) // 4 default DNS names + additional SANs

			// Verify all additional SANs are included
			for _, san := range additionalSANs {
				Expect(mockCert.LastDnsNames).To(ContainElement(san))
			}
		})
	})
})
