package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"golang.org/x/crypto/bcrypt"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v2"
)

func newSecurityconfigReconciler(
	client *k8s.MockK8sClient,
	ctx context.Context,
	reconcilerContext *ReconcilerContext,
	instance *opensearchv1.OpenSearchCluster,
) *SecurityconfigReconciler {
	return &SecurityconfigReconciler{
		client:            client,
		ctx:               ctx,
		reconcilerContext: reconcilerContext,
		recorder:          &helpers.MockEventRecorder{},
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

var _ = Describe("Securityconfig Reconciler", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName    = "securityconfig"
		adminCredsName = "admin-creds"

		defaultAdminHash          = "$2y$12$lJsHWchewGVcGlYgE3js/O4bkTZynETyXChAITarCHLz8cuaueIyq"
		defaultKibanaServerHash   = "$2a$12$4AcgAt3xwOWadA5s5blL6ev39OXDNhmOesEoo33eZtrq2N0YrU3H."
		internalUsersTemplateYAML = `_meta:
  type: "internalusers"
  config_version: 2
admin:
  hash: "%s"
  reserved: true
  backend_roles:
    - "admin"
  description: "Demo admin user"
kibanaserver:
  hash: "%s"
  reserved: true
  description: "Demo user for the OpenSearch Dashboards server"
`
		configYAML = `_meta:
  type: "config"
  config_version: "2"
config:
  dynamic:
    http:
      anonymous_auth_enabled: false
`
		actionGroupsYAML = `_meta:
  type: "actiongroups"
  config_version: 2
`
	)

	internalUsersYAML := func(adminHash, kibanaHash string) []byte {
		if adminHash == "" {
			adminHash = defaultAdminHash
		}
		if kibanaHash == "" {
			kibanaHash = defaultKibanaServerHash
		}
		return []byte(fmt.Sprintf(internalUsersTemplateYAML, adminHash, kibanaHash))
	}

	newAdminCredentialsSecret := func(namespace string) corev1.Secret {
		return corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      adminCredsName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("changeme"),
			},
		}
	}

	// setupDashboardsCredentialsSecretMocks sets up mocks for dashboards credentials secret creation
	setupDashboardsCredentialsSecretMocks := func(mockClient *k8s.MockK8sClient, clusterName string) {
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
	}

	When("When Reconciling the securityconfig reconciler with no securityconfig provided in the spec", func() {
		It("should not do anything", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opensearchv1.ClusterSpec{General: opensearchv1.GeneralConfig{}},
			}

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			result, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())
		})
	})

	When("When Reconciling the securityconfig reconciler with securityconfig secret configured and available and tls configured", func() {
		It("should start an update job only apply ymls present in secret", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			adminCredSecret := newAdminCredentialsSecret(clusterName)
			existingHash, err := bcrypt.GenerateFromPassword([]byte("changeme"), bcrypt.MinCost)
			Expect(err).ToNot(HaveOccurred())
			securityConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				Data: map[string][]byte{
					"config.yml":         []byte(configYAML),
					"internal_users.yml": internalUsersYAML("", ""),
					// Invalid yml in secret should not throw an error
					"invalid.yml": []byte("foo"),
					// Non-yml files in secret should be skipped without an error
					"log4j2.properties": []byte("rootLogger.level = info"),
					// Empty contents for a yml should be ignored
					"action_groups.yml": []byte(actionGroupsYAML),
				},
			}
			existingInternalUsers := internalUsersYAML(string(existingHash), defaultKibanaServerHash)
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.3",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{
							SecurityconfigSecret:   corev1.LocalObjectReference{Name: "securityconfig-secret"},
							AdminCredentialsSecret: corev1.LocalObjectReference{Name: adminCredsName},
						},
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{Generate: true},
							Http:      &opensearchv1.TlsConfigHttp{Generate: true},
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					Initialized: true,
				},
			}
			generatedConfigName := helpers.GeneratedSecurityConfigSecretName(&spec)
			mockClient.EXPECT().GetSecret(adminCredsName, clusterName).Return(adminCredSecret, nil)
			mockClient.EXPECT().GetSecret("securityconfig-secret", clusterName).Return(*securityConfigSecret, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			mockClient.On("GetSecret", generatedConfigName, clusterName).
				Return(corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: generatedConfigName, Namespace: clusterName},
					Data:       map[string][]byte{"internal_users.yml": existingInternalUsers},
				}, nil).Once()
			mockClient.EXPECT().GetJob("securityconfig-securityconfig-update", clusterName).Return(batchv1.Job{}, NotFoundError())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil).Maybe()

			var generatedConfigSecret *corev1.Secret
			mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).
				Return(&ctrl.Result{}, nil).
				Run(func(args mock.Arguments) {
					if secret, ok := args[0].(*corev1.Secret); ok && secret.Name == generatedConfigName {
						generatedConfigSecret = secret.DeepCopy()
					}
				})
			mockClient.On("GetSecret", generatedConfigName, clusterName).
				Return(func(string, string) corev1.Secret {
					Expect(generatedConfigSecret).ToNot(BeNil())
					return *generatedConfigSecret
				}, nil).Once()

			var createdJob *batchv1.Job
			mockClient.On("CreateJob", mock.Anything).
				Return(func(job *batchv1.Job) (*ctrl.Result, error) {
					createdJob = job
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			job := *createdJob

			Expect(generatedConfigSecret).ToNot(BeNil())
			var internalCfg helpers.InternalUserConfig
			Expect(yaml.Unmarshal(generatedConfigSecret.Data["internal_users.yml"], &internalCfg)).To(Succeed())
			Expect(internalCfg.Admin.Hash).To(Equal(string(existingHash)))

			actualCmdArg := job.Spec.Template.Spec.Containers[0].Args[0]
			// Verify that expected files were present in the command
			Expect(actualCmdArg).To(ContainSubstring("config.yml"))
			Expect(actualCmdArg).To(ContainSubstring("internal_users.yml"))
			Expect(actualCmdArg).To(ContainSubstring("action_groups.yml"))
			// Verify that invalid files were not included in the command
			Expect(actualCmdArg).ToNot(ContainSubstring("invalid.yml"))
			// Verify that non-yml files were not included in the command
			Expect(actualCmdArg).ToNot(ContainSubstring("log4j2.properties"))
			// Verify that files not present in the secret are not included
			Expect(actualCmdArg).ToNot(ContainSubstring("audit.yml"))
		})
	})

	When("When Reconciling the securityconfig reconciler with securityconfig secret but no TLS configured", func() {
		It("should not start an update job", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			var clusterName = "securityconfig-notls"
			securityConfigSecretName := clusterName + "-security-config"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.3",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: securityConfigSecretName},
						},
						// No TLS configured - security plugin is disabled
					},
				},
			}
			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			// Note: Not creating the update job is verified implicitly because the reconciler exits early when TLS is not configured (security plugin disabled)
		})
	})

	When("When Reconciling the securityconfig reconciler with no securityconfig secret but tls configured", func() {
		It("should start an update job and apply all yml files", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			var clusterName = "no-securityconfig-tls-configured"

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.3",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{},
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{Generate: true},
							Http:      &opensearchv1.TlsConfigHttp{Generate: true},
						},
					},
				},
			}
			generatedAdminName := helpers.GeneratedAdminCredentialsSecretName(&spec)
			generatedConfigName := helpers.GeneratedSecurityConfigSecretName(&spec)
			autoAdminSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      generatedAdminName,
					Namespace: clusterName,
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("auto-generated"),
				},
			}

			mockClient.On("GetSecret", generatedAdminName, clusterName).Return(corev1.Secret{}, NotFoundError()).Once()
			mockClient.On("CreateSecret", mock.MatchedBy(func(secret *corev1.Secret) bool {
				return secret.Name == generatedAdminName
			})).Return(&ctrl.Result{}, nil)
			mockClient.On("GetSecret", generatedAdminName, clusterName).Return(autoAdminSecret, nil).Once()
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			mockClient.On("GetSecret", generatedConfigName, clusterName).Return(corev1.Secret{}, NotFoundError()).Once()

			mockClient.EXPECT().GetJob("no-securityconfig-tls-configured-securityconfig-update", clusterName).Return(batchv1.Job{}, NotFoundError())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil).Maybe()

			var generatedConfigSecret *corev1.Secret
			mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).
				Return(&ctrl.Result{}, nil).
				Run(func(args mock.Arguments) {
					if secret, ok := args[0].(*corev1.Secret); ok && secret.Name == generatedConfigName {
						generatedConfigSecret = secret.DeepCopy()
					}
				})
			mockClient.On("GetSecret", generatedConfigName, clusterName).
				Return(func(string, string) corev1.Secret {
					Expect(generatedConfigSecret).ToNot(BeNil())
					return *generatedConfigSecret
				}, nil)

			var createdJob *batchv1.Job
			mockClient.On("CreateJob", mock.Anything).
				Return(func(job *batchv1.Job) (*ctrl.Result, error) {
					createdJob = job
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(createdJob).ToNot(BeNil())

			cmdArg := `ADMIN=/usr/share/opensearch/plugins/opensearch-security/tools/securityadmin.sh;
chmod +x $ADMIN;
wait_count=0;
until curl -k --silent https://no-securityconfig-tls-configured.no-securityconfig-tls-configured.svc.cluster.local:9200;
do
  if (( wait_count++ >= 60 )); then
    echo "Failed to connect to cluster after 60 attempts";
    exit 1;
  fi;
  echo 'Waiting to connect to the cluster'; sleep 20;
done;count=0;
until $ADMIN -cacert /certs/ca.crt -cert /certs/tls.crt -key /certs/tls.key -cd /usr/share/opensearch/config/opensearch-security -icl -nhnv -h no-securityconfig-tls-configured.no-securityconfig-tls-configured.svc.cluster.local -p 9200; do
  if (( count++ >= 20 )); then
    echo "Failed to apply securityconfig after 20 attempts";
    exit 1;
  fi;
  sleep 20;
done;`

			Expect(createdJob.Spec.Template.Spec.Containers[0].Args[0]).To(Equal(cmdArg))
		})
	})

	When("Reconciling with a failed securityconfig update job", func() {
		buildReconcileFixture := func(mockClient *k8s.MockK8sClient) (*opensearchv1.OpenSearchCluster, **corev1.Secret) {
			adminCredSecret := newAdminCredentialsSecret(clusterName)
			securityConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				Data: map[string][]byte{
					"config.yml":         []byte(configYAML),
					"internal_users.yml": internalUsersYAML("", ""),
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.3",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{
							SecurityconfigSecret:   corev1.LocalObjectReference{Name: "securityconfig-secret"},
							AdminCredentialsSecret: corev1.LocalObjectReference{Name: adminCredsName},
						},
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{Generate: true},
							Http:      &opensearchv1.TlsConfigHttp{Generate: true},
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					Initialized:          true,
					ContextSecretCreated: true,
				},
			}
			generatedConfigName := helpers.GeneratedSecurityConfigSecretName(&spec)

			mockClient.EXPECT().GetSecret(adminCredsName, clusterName).Return(adminCredSecret, nil)
			mockClient.EXPECT().GetSecret("securityconfig-secret", clusterName).Return(*securityConfigSecret, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			mockClient.On("GetSecret", generatedConfigName, clusterName).
				Return(corev1.Secret{}, NotFoundError()).Once()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil).Maybe()

			var generatedConfigSecret *corev1.Secret
			mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).
				Return(&ctrl.Result{}, nil).
				Run(func(args mock.Arguments) {
					if secret, ok := args[0].(*corev1.Secret); ok && secret.Name == generatedConfigName {
						generatedConfigSecret = secret.DeepCopy()
					}
				})
			mockClient.On("GetSecret", generatedConfigName, clusterName).
				Return(func(string, string) corev1.Secret {
					Expect(generatedConfigSecret).ToNot(BeNil())
					return *generatedConfigSecret
				}, nil).Once()

			return &spec, &generatedConfigSecret
		}

		jobWithStatus := func(generatedConfigSecret **corev1.Secret, status batchv1.JobStatus) batchv1.Job {
			Expect(*generatedConfigSecret).ToNot(BeNil())
			checksumval, err := checksum((*generatedConfigSecret).Data)
			Expect(err).ToNot(HaveOccurred())
			return batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "securityconfig-securityconfig-update",
					Namespace:   clusterName,
					Annotations: map[string]string{checksumAnnotation: checksumval},
				},
				Status: status,
			}
		}

		It("should not recreate the job when it already succeeded", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec, generatedConfigSecret := buildReconcileFixture(mockClient)
			mockClient.EXPECT().GetJob("securityconfig-securityconfig-update", clusterName).
				RunAndReturn(func(string, string) (batchv1.Job, error) {
					return jobWithStatus(generatedConfigSecret, batchv1.JobStatus{Succeeded: 1}), nil
				})

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(mockClient, context.Background(), &reconcilerContext, spec)
			result, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())
		})

		It("should requeue while the update job is still running", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec, generatedConfigSecret := buildReconcileFixture(mockClient)
			mockClient.EXPECT().GetJob("securityconfig-securityconfig-update", clusterName).
				RunAndReturn(func(string, string) (batchv1.Job, error) {
					return jobWithStatus(generatedConfigSecret, batchv1.JobStatus{Active: 1}), nil
				})

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(mockClient, context.Background(), &reconcilerContext, spec)
			result, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(30 * time.Second))
		})

		It("should delete and recreate the job when it failed with a matching checksum", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec, generatedConfigSecret := buildReconcileFixture(mockClient)
			mockClient.EXPECT().GetJob("securityconfig-securityconfig-update", clusterName).
				RunAndReturn(func(string, string) (batchv1.Job, error) {
					return jobWithStatus(generatedConfigSecret, batchv1.JobStatus{Failed: 1}), nil
				})
			mockClient.EXPECT().DeleteJob(mock.AnythingOfType("*v1.Job")).Return(nil)

			var createdJob *batchv1.Job
			mockClient.On("CreateJob", mock.Anything).
				Return(func(job *batchv1.Job) (*ctrl.Result, error) {
					createdJob = job
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(mockClient, context.Background(), &reconcilerContext, spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(createdJob).ToNot(BeNil())
		})
	})

	Describe("securityConfigRetryDelay", func() {
		It("should use exponential backoff capped at the maximum delay", func() {
			Expect(securityConfigRetryDelay(0)).To(Equal(30 * time.Second))
			Expect(securityConfigRetryDelay(1)).To(Equal(60 * time.Second))
			Expect(securityConfigRetryDelay(2)).To(Equal(120 * time.Second))
			Expect(securityConfigRetryDelay(10)).To(Equal(15 * time.Minute))
		})
	})

	When("Determining admin CA secret for securityconfig update job", func() {
		It("should use HTTP caSecret for security change versions", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ca-http", Namespace: "ca-http", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.3.0",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Http: &opensearchv1.TlsConfigHttp{
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									CaSecret: corev1.LocalObjectReference{Name: "http-ca"},
								},
							},
						},
					},
				},
			}
			underTest := &SecurityconfigReconciler{instance: &spec}
			Expect(underTest.determineAdminCASecret("admin-secret")).To(Equal("http-ca"))
		})

		It("should return empty when CA secret equals admin secret", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "same-ca", Namespace: "same-ca", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.3.0",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Http: &opensearchv1.TlsConfigHttp{
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									CaSecret: corev1.LocalObjectReference{Name: "admin-secret"},
								},
							},
						},
					},
				},
			}
			underTest := &SecurityconfigReconciler{instance: &spec}
			Expect(underTest.determineAdminCASecret("admin-secret")).To(BeEmpty())
		})

		It("should use transport caSecret for pre-2.0 versions", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ca-transport", Namespace: "ca-transport", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "1.3.0",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									CaSecret: corev1.LocalObjectReference{Name: "transport-ca"},
								},
							},
						},
					},
				},
			}
			underTest := &SecurityconfigReconciler{instance: &spec}
			Expect(underTest.determineAdminCASecret("admin-secret")).To(Equal("transport-ca"))
		})
	})

	When("Reconciling with external TLS certs and separate caSecret", func() {
		const (
			externalClusterName = "external-tls"
			tlsSecretName       = "my-tls-secret"
			caSecretName        = "my-ca-secret"
		)

		findJobVolume := func(job batchv1.Job, name string) corev1.Volume {
			for _, volume := range job.Spec.Template.Spec.Volumes {
				if volume.Name == name {
					return volume
				}
			}
			return corev1.Volume{}
		}

		It("should project admin-cert from TLS secret and caSecret when reconciling", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			adminCredSecret := newAdminCredentialsSecret(externalClusterName)
			securityConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: externalClusterName},
				Type:       corev1.SecretType("Opaque"),
				Data: map[string][]byte{
					"config.yml": []byte(configYAML),
				},
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: externalClusterName, Namespace: externalClusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: externalClusterName,
						Version:     "3.5.0",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{
							SecurityconfigSecret:   corev1.LocalObjectReference{Name: "securityconfig-secret"},
							AdminCredentialsSecret: corev1.LocalObjectReference{Name: adminCredsName},
							AdminSecret:            corev1.LocalObjectReference{Name: tlsSecretName},
						},
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{
								Generate: false,
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									Secret:   corev1.LocalObjectReference{Name: tlsSecretName},
									CaSecret: corev1.LocalObjectReference{Name: caSecretName},
								},
							},
							Http: &opensearchv1.TlsConfigHttp{
								Generate: false,
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									Secret:   corev1.LocalObjectReference{Name: tlsSecretName},
									CaSecret: corev1.LocalObjectReference{Name: caSecretName},
								},
							},
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					Initialized: true,
				},
			}
			generatedConfigName := helpers.GeneratedSecurityConfigSecretName(&spec)
			mockClient.EXPECT().GetSecret(adminCredsName, externalClusterName).Return(adminCredSecret, nil)
			mockClient.EXPECT().GetSecret("securityconfig-secret", externalClusterName).Return(*securityConfigSecret, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, externalClusterName)
			mockClient.EXPECT().GetJob(externalClusterName+"-securityconfig-update", externalClusterName).Return(batchv1.Job{}, NotFoundError())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil).Maybe()
			mockClient.On("GetSecret", generatedConfigName, externalClusterName).Return(corev1.Secret{}, NotFoundError()).Once()

			var generatedConfigSecret *corev1.Secret
			mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).
				Return(&ctrl.Result{}, nil).
				Run(func(args mock.Arguments) {
					if secret, ok := args[0].(*corev1.Secret); ok && secret.Name == generatedConfigName {
						generatedConfigSecret = secret.DeepCopy()
					}
				})
			mockClient.On("GetSecret", generatedConfigName, externalClusterName).
				Return(func(string, string) corev1.Secret {
					Expect(generatedConfigSecret).ToNot(BeNil())
					return *generatedConfigSecret
				}, nil).Once()

			var createdJob *batchv1.Job
			mockClient.On("CreateJob", mock.Anything).
				Return(func(job *batchv1.Job) (*ctrl.Result, error) {
					createdJob = job
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(createdJob).ToNot(BeNil())

			adminVolume := findJobVolume(*createdJob, "admin-cert")
			Expect(adminVolume.Projected).ToNot(BeNil())
			Expect(adminVolume.Projected.Sources).To(HaveLen(2))
			Expect(adminVolume.Projected.Sources[0].Secret.Name).To(Equal(tlsSecretName))
			Expect(adminVolume.Projected.Sources[0].Secret.Items).To(ConsistOf(
				corev1.KeyToPath{Key: corev1.TLSCertKey, Path: corev1.TLSCertKey},
				corev1.KeyToPath{Key: corev1.TLSPrivateKeyKey, Path: corev1.TLSPrivateKeyKey},
			))
			Expect(adminVolume.Projected.Sources[1].Secret.Name).To(Equal(caSecretName))
			Expect(adminVolume.Projected.Sources[1].Secret.Items).To(ConsistOf(
				corev1.KeyToPath{Key: "ca.crt", Path: "ca.crt"},
			))

			cmdArg := createdJob.Spec.Template.Spec.Containers[0].Args[0]
			Expect(cmdArg).To(ContainSubstring("-cacert /certs/ca.crt"))
			Expect(cmdArg).To(ContainSubstring("-cert /certs/tls.crt"))
			Expect(cmdArg).To(ContainSubstring("-key /certs/tls.key"))
		})
	})
	When("When Reconciling the securityconfig reconciler with transport TLS enabled but HTTP TLS disabled", func() {
		It("should apply the securityconfig via default init instead of a securityadmin job", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			var clusterName = "securityconfig-default-init"

			adminCredSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: adminCredsName, Namespace: clusterName},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("changeme"),
				},
			}
			httpTlsDisabled := false
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.19.4",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{
							AdminCredentialsSecret: corev1.LocalObjectReference{Name: adminCredsName},
						},
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{Generate: true},
							Http:      &opensearchv1.TlsConfigHttp{Enabled: &httpTlsDisabled},
						},
					},
				},
			}
			generatedConfigName := helpers.GeneratedSecurityConfigSecretName(&spec)
			mockClient.EXPECT().GetSecret(adminCredsName, clusterName).Return(adminCredSecret, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			mockClient.On("GetSecret", generatedConfigName, clusterName).Return(corev1.Secret{}, NotFoundError()).Once()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil)

			var generatedConfigSecret *corev1.Secret
			mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).
				Return(&ctrl.Result{}, nil).
				Run(func(args mock.Arguments) {
					if secret, ok := args[0].(*corev1.Secret); ok && secret.Name == generatedConfigName {
						generatedConfigSecret = secret.DeepCopy()
					}
				})
			mockClient.On("GetSecret", generatedConfigName, clusterName).
				Return(func(string, string) corev1.Secret {
					Expect(generatedConfigSecret).ToNot(BeNil())
					return *generatedConfigSecret
				}, nil).Once()

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			result, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())

			Expect(reconcilerContext.OpenSearchConfig).To(HaveKeyWithValue("plugins.security.allow_default_init_securityindex", "true"))

			Expect(generatedConfigSecret).ToNot(BeNil())
			var mountPaths []string
			for _, mount := range reconcilerContext.VolumeMounts {
				if mount.Name == "securityconfig" {
					mountPaths = append(mountPaths, mount.MountPath)
				}
			}
			Expect(mountPaths).ToNot(BeEmpty())
			Expect(mountPaths).To(ContainElement(ContainSubstring("internal_users.yml")))
		})
	})

	When("Reconciling an initialized cluster whose applied securityconfig is recorded", func() {
		const (
			userSecretName = "securityconfig-secret"
			jobName        = clusterName + "-securityconfig-update"
		)

		var (
			mockClient *k8s.MockK8sClient
			transport  *httpmock.MockTransport
			spec       *opensearchv1.OpenSearchCluster
			userData   map[string][]byte
			adminHash  string
			kibanaHash string
			generated  *corev1.Secret
			createdJob *batchv1.Job
		)

		BeforeEach(func() {
			mockClient = k8s.NewMockK8sClient(GinkgoT())
			transport = httpmock.NewMockTransport()
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL)
			})
			// Hashes that match the admin and dashboards passwords, so the operator keeps them.
			hash, err := bcrypt.GenerateFromPassword([]byte("changeme"), bcrypt.MinCost)
			Expect(err).ToNot(HaveOccurred())
			adminHash = string(hash)
			hash, err = bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
			Expect(err).ToNot(HaveOccurred())
			kibanaHash = string(hash)

			userData = map[string][]byte{
				"config.yml":         []byte(configYAML),
				"internal_users.yml": internalUsersYAML("", ""),
			}
			spec = &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.3",
					},
					Security: &opensearchv1.Security{
						Config: &opensearchv1.SecurityConfig{
							SecurityconfigSecret:   corev1.LocalObjectReference{Name: userSecretName},
							AdminCredentialsSecret: corev1.LocalObjectReference{Name: adminCredsName},
						},
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{Generate: true},
							Http:      &opensearchv1.TlsConfigHttp{Generate: true},
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					Initialized:          true,
					ContextSecretCreated: true,
				},
			}
			generated = nil
			createdJob = nil
			transport.RegisterResponder(http.MethodHead, helpers.ClusterURL(spec)+"/",
				httpmock.NewStringResponder(http.StatusOK, ""))
			transport.RegisterResponder(http.MethodGet, helpers.ClusterURL(spec)+"/",
				httpmock.NewStringResponder(http.StatusOK, `{"version":{"number":"2.3.0","distribution":"opensearch"}}`))
		})

		// expectedState is the state the reconciler computes for userData.
		expectedState := func() opensearchv1.SecurityConfigStatus {
			source, err := helpers.LoadSecurityConfigSource(nil, &opensearchv1.OpenSearchCluster{})
			Expect(err).ToNot(HaveOccurred())
			for name, content := range userData {
				source.Data[name] = content
				source.UserKeys[name] = true
			}
			state, err := securityConfigState(source, &corev1.Secret{
				Data: map[string][]byte{"internal_users.yml": internalUsersYAML(adminHash, kibanaHash)},
			})
			Expect(err).ToNot(HaveOccurred())
			return state
		}

		appliedWith := func(change func(*opensearchv1.SecurityConfigStatus)) *opensearchv1.SecurityConfigStatus {
			state := expectedState()
			state.UpdateJobChecksum = "previous-job"
			change(&state)
			return &state
		}

		// reconcile runs the reconciler with the given applied state. jobFor returns the existing
		// update job, or nil when there is none.
		reconcile := func(applied *opensearchv1.SecurityConfigStatus, jobFor func(generated *corev1.Secret) *batchv1.Job) (ctrl.Result, error) {
			spec.Status.SecurityConfig = applied
			generatedName := helpers.GeneratedSecurityConfigSecretName(spec)

			mockClient.EXPECT().GetSecret(adminCredsName, clusterName).Return(newAdminCredentialsSecret(clusterName), nil)
			mockClient.EXPECT().GetSecret(userSecretName, clusterName).Return(corev1.Secret{Data: userData}, nil)
			setupDashboardsCredentialsSecretMocks(mockClient, clusterName)
			mockClient.On("GetSecret", generatedName, clusterName).Return(corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: generatedName, Namespace: clusterName},
				Data:       map[string][]byte{"internal_users.yml": internalUsersYAML(adminHash, kibanaHash)},
			}, nil).Once()
			mockClient.On("ReconcileResource", mock.AnythingOfType("*v1.Secret"), mock.Anything).
				Return(&ctrl.Result{}, nil).
				Run(func(args mock.Arguments) {
					if secret, ok := args[0].(*corev1.Secret); ok && secret.Name == generatedName {
						generated = secret.DeepCopy()
					}
				})
			mockClient.On("GetSecret", generatedName, clusterName).
				Return(func(string, string) corev1.Secret { return *generated }, nil).Once()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil).Maybe()
			mockClient.EXPECT().GetJob(jobName, clusterName).RunAndReturn(func(string, string) (batchv1.Job, error) {
				if jobFor == nil {
					return batchv1.Job{}, NotFoundError()
				}
				return *jobFor(generated), nil
			})
			mockClient.On("DeleteJob", mock.AnythingOfType("*v1.Job")).Return(nil).Maybe()
			mockClient.On("CreateJob", mock.Anything).Return(func(job *batchv1.Job) (*ctrl.Result, error) {
				createdJob = job
				return &ctrl.Result{}, nil
			}).Maybe()

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(mockClient, context.Background(), &reconcilerContext, spec)
			underTest.options.osClientTransport = transport
			return underTest.Reconcile()
		}

		It("should only re-apply the user-supplied files that changed", func() {
			_, err := reconcile(appliedWith(func(s *opensearchv1.SecurityConfigStatus) {
				s.AppliedChecksums["config.yml"] = "old"
			}), nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(createdJob).ToNot(BeNil())
			cmdArg := createdJob.Spec.Template.Spec.Containers[0].Args[0]
			Expect(cmdArg).To(ContainSubstring("config.yml -t config"))
			Expect(cmdArg).ToNot(ContainSubstring("internal_users.yml"))
			Expect(cmdArg).ToNot(ContainSubstring("tenants.yml"))
			Expect(cmdArg).ToNot(ContainSubstring("-cd "))
			target, ok := jobAppliedState(*createdJob)
			Expect(ok).To(BeTrue())
			Expect(securityConfigStatesEqual(target, expectedState())).To(BeTrue())
		})

		It("should not re-apply bundled defaults that changed", func() {
			delete(userData, "internal_users.yml")
			result, err := reconcile(appliedWith(func(s *opensearchv1.SecurityConfigStatus) {
				s.AppliedChecksums["internal_users.yml"] = "old"
			}), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())

			Expect(createdJob).To(BeNil())
			Expect(securityConfigStatesEqual(*spec.Status.SecurityConfig, expectedState())).To(BeTrue())
			Expect(spec.Status.SecurityConfig.UpdateJobChecksum).To(Equal("previous-job"))
		})

		It("should not recreate a deleted update job when nothing changed", func() {
			result, err := reconcile(appliedWith(func(*opensearchv1.SecurityConfigStatus) {}), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())
			Expect(createdJob).To(BeNil())
		})

		It("should update rotated password hashes through the REST API instead of re-applying internal_users.yml", func() {
			clusterURL := helpers.ClusterURL(spec) + "/"
			patchedHashes := map[string]string{}
			for _, name := range []string{"admin", "kibanaserver"} {
				transport.RegisterResponder(http.MethodPatch, clusterURL+"_plugins/_security/api/internalusers/"+name,
					func(req *http.Request) (*http.Response, error) {
						var ops []map[string]string
						Expect(json.NewDecoder(req.Body).Decode(&ops)).To(Succeed())
						Expect(ops).To(HaveLen(1))
						Expect(ops[0]).To(HaveKeyWithValue("op", "add"))
						Expect(ops[0]).To(HaveKeyWithValue("path", "/hash"))
						patchedHashes[name] = ops[0]["value"]
						return httpmock.NewStringResponse(http.StatusOK, `{"status":"OK"}`), nil
					})
			}

			result, err := reconcile(appliedWith(func(s *opensearchv1.SecurityConfigStatus) {
				s.ManagedUsersChecksum = "old"
			}), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())

			Expect(createdJob).To(BeNil())
			Expect(patchedHashes).To(Equal(map[string]string{"admin": adminHash, "kibanaserver": kibanaHash}))
			Expect(spec.Status.SecurityConfig.ManagedUsersChecksum).To(Equal(expectedState().ManagedUsersChecksum))
		})

		It("should retry later when updating the password hashes fails", func() {
			clusterURL := helpers.ClusterURL(spec) + "/"
			transport.RegisterResponder(http.MethodPatch, clusterURL+"_plugins/_security/api/internalusers/admin",
				httpmock.NewStringResponder(http.StatusForbidden, `{"status":"FORBIDDEN"}`))

			result, err := reconcile(appliedWith(func(s *opensearchv1.SecurityConfigStatus) {
				s.ManagedUsersChecksum = "old"
			}), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(30 * time.Second))
			Expect(createdJob).To(BeNil())
			Expect(spec.Status.SecurityConfig.ManagedUsersChecksum).To(Equal("old"))
		})

		It("should re-apply a file a succeeded job changed after it was changed back", func() {
			applied := appliedWith(func(*opensearchv1.SecurityConfigStatus) {})
			jobTarget := appliedWith(func(s *opensearchv1.SecurityConfigStatus) {
				s.AppliedChecksums["config.yml"] = "changed-and-reverted"
			})
			_, err := reconcile(applied, func(*corev1.Secret) *batchv1.Job {
				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:        jobName,
						Namespace:   clusterName,
						Annotations: map[string]string{checksumAnnotation: "other"},
					},
					Status: batchv1.JobStatus{Succeeded: 1},
				}
				Expect(setJobAppliedState(job, *jobTarget)).To(Succeed())
				return job
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(createdJob).ToNot(BeNil())
			cmdArg := createdJob.Spec.Template.Spec.Containers[0].Args[0]
			Expect(cmdArg).To(ContainSubstring("config.yml -t config"))
			Expect(cmdArg).ToNot(ContainSubstring("internal_users.yml"))
		})

		It("should record the state applied by a job of an earlier operator version", func() {
			result, err := reconcile(nil, func(generated *corev1.Secret) *batchv1.Job {
				checksumval, err := checksum(generated.Data)
				Expect(err).ToNot(HaveOccurred())
				return &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:        jobName,
						Namespace:   clusterName,
						Annotations: map[string]string{checksumAnnotation: checksumval},
					},
					Status: batchv1.JobStatus{Succeeded: 1},
				}
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())

			Expect(createdJob).To(BeNil())
			Expect(spec.Status.SecurityConfig).ToNot(BeNil())
			Expect(securityConfigStatesEqual(*spec.Status.SecurityConfig, expectedState())).To(BeTrue())
		})
	})

	Describe("planSecurityConfigUpdate", func() {
		userKeys := map[string]bool{"config.yml": true, "internal_users.yml": true}
		applied := func() opensearchv1.SecurityConfigStatus {
			return opensearchv1.SecurityConfigStatus{
				AppliedChecksums:     map[string]string{"config.yml": "a", "internal_users.yml": "b", "tenants.yml": "c"},
				ManagedUsersChecksum: "m",
			}
		}

		It("should re-apply changed user files and skip changed bundled defaults", func() {
			current := applied()
			current.AppliedChecksums["config.yml"] = "a2"
			current.AppliedChecksums["tenants.yml"] = "c2"
			current.AppliedChecksums["roles.yml"] = "new"

			plan := planSecurityConfigUpdate(applied(), current, map[string]bool{"config.yml": true, "roles.yml": true}, nil)
			Expect(plan.files).To(Equal([]string{"config.yml", "roles.yml"}))
			Expect(plan.skippedDefaults).To(Equal([]string{"tenants.yml"}))
			Expect(plan.patchManagedUsers).To(BeFalse())
		})

		It("should patch the managed password hashes when internal_users.yml is not re-applied", func() {
			current := applied()
			current.ManagedUsersChecksum = "m2"

			plan := planSecurityConfigUpdate(applied(), current, userKeys, nil)
			Expect(plan.files).To(BeEmpty())
			Expect(plan.patchManagedUsers).To(BeTrue())
		})

		It("should not patch the managed password hashes when internal_users.yml is re-applied", func() {
			current := applied()
			current.ManagedUsersChecksum = "m2"
			current.AppliedChecksums["internal_users.yml"] = "b2"

			plan := planSecurityConfigUpdate(applied(), current, userKeys, nil)
			Expect(plan.files).To(Equal([]string{"internal_users.yml"}))
			Expect(plan.patchManagedUsers).To(BeFalse())
		})

		It("should re-apply files an unfinished job may have applied", func() {
			plan := planSecurityConfigUpdate(applied(), applied(), userKeys, []string{"config.yml", "roles.yml"})
			Expect(plan.files).To(Equal([]string{"config.yml"}))
		})
	})
})
