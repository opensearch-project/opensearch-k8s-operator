package reconcilers

import (
	"context"
	"fmt"

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
until curl -k --silent https://no-securityconfig-tls-configured.no-securityconfig-tls-configured.svc.cluster.local:9200;
do
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
})
