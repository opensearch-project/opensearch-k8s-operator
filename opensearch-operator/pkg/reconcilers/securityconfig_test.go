package reconcilers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

func newSecurityconfigReconciler(
	client *k8s.MockK8sClient,
	ctx context.Context,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
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
		clusterName = "securityconfig"
	)

	When("When Reconciling the securityconfig reconciler with no securityconfig provided in the spec", func() {
		It("should not do anything", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}},
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

	When("When Reconciling the securityconfig reconciler with securityconfig secret configured but not available", func() {
		It("should trigger a requeue", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "foobar"},
							AdminSecret:          corev1.LocalObjectReference{Name: "admin"},
						},
					},
				}}
			mockClient.EXPECT().GetSecret("foobar", clusterName).Return(corev1.Secret{}, NotFoundError())

			reconcilerContext := NewReconcilerContext(&record.FakeRecorder{}, &spec, spec.Spec.NodePools)
			underTest := newSecurityconfigReconciler(
				mockClient,
				context.Background(),
				&reconcilerContext,
				&spec,
			)
			result, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.IsZero()).To(BeFalse())
			Expect(result.Requeue).To(BeTrue())
		})
	})

	When("When Reconciling the securityconfig reconciler with securityconfig secret configured and available and tls configured", func() {
		It("should start an update job only apply ymls present in secret", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			securityConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				Data: map[string][]byte{
					"config.yml":         []byte("foobar"),
					"internal_users.yml": []byte("bar"),
					// Invalid yml in secret should not throw an error
					"invalid.yml": []byte("foo"),
					// Empty contents for a yml should be ignored
					"action_groups.yml": []byte(""),
				},
			}
			mockClient.EXPECT().GetSecret("securityconfig-secret", clusterName).Return(*securityConfigSecret, nil)
			mockClient.EXPECT().GetJob("securityconfig-securityconfig-update", clusterName).Return(batchv1.Job{}, NotFoundError())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)

			var createdJob *batchv1.Job
			mockClient.On("CreateJob", mock.Anything).
				Return(func(job *batchv1.Job) (*ctrl.Result, error) {
					createdJob = job
					return &ctrl.Result{}, nil
				})

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.3",
					},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig-secret"},
						},
						Tls: &opsterv1.TlsConfig{
							Transport: &opsterv1.TlsConfigTransport{Generate: true},
						},
					},
				},
				Status: opsterv1.ClusterStatus{
					Initialized: true,
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

			job := *createdJob

			actualCmdArg := job.Spec.Template.Spec.Containers[0].Args[0]
			// Verify that expected files were present in the command
			Expect(actualCmdArg).To(ContainSubstring("config.yml"))
			Expect(actualCmdArg).To(ContainSubstring("internal_users.yml"))
			// Verify that invalid files were not included in the command
			Expect(actualCmdArg).ToNot(ContainSubstring("invalid.yml"))
			// Verify that empty files were not included in the command
			Expect(actualCmdArg).ToNot(ContainSubstring("action_groups.yml"))
			// Verify that files not present in the secret are not included
			Expect(actualCmdArg).ToNot(ContainSubstring("audit.yml"))
		})
	})

	When("When Reconciling the securityconfig reconciler with both securityconfig and admin secret configured and available but no tls configured", func() {
		It("should start an update job", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			var clusterName = "securityconfig-withadminsecret"

			securityConfigSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				Data:       map[string][]byte{},
			}
			mockClient.EXPECT().GetSecret("securityconfig-secret", clusterName).Return(securityConfigSecret, nil)
			mockClient.EXPECT().GetJob("securityconfig-withadminsecret-securityconfig-update", clusterName).Return(batchv1.Job{}, NotFoundError())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)

			var createdJob *batchv1.Job
			mockClient.On("CreateJob", mock.Anything).
				Return(func(job *batchv1.Job) (*ctrl.Result, error) {
					createdJob = job
					return &ctrl.Result{}, nil
				})

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						Version: "2.3",
					},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig-secret"},
							AdminSecret:          corev1.LocalObjectReference{Name: "admin-cert"},
						},
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
			Expect(createdJob).ToNot(BeNil())
		})
	})

	When("When Reconciling the securityconfig reconciler with securityconfig secret but no adminSecret configured", func() {
		It("should not start an update job", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			var clusterName = "securityconfig-noadminsecret"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig"},
						},
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
			// Note: Not creating the update job is verified implicitly because the test would fail if any of the mock methods are called
		})
	})

	When("When Reconciling the securityconfig reconciler with no securityconfig secret but tls configured", func() {
		It("should start an update job and apply all yml files", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			var clusterName = "no-securityconfig-tls-configured"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName: clusterName,
						Version:     "2.3",
					},
					Security: &opsterv1.Security{
						Tls: &opsterv1.TlsConfig{
							Transport: &opsterv1.TlsConfigTransport{Generate: true},
						},
					},
				},
			}

			mockClient.EXPECT().GetJob("no-securityconfig-tls-configured-securityconfig-update", clusterName).Return(batchv1.Job{}, NotFoundError())
			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
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
echo 'Waiting to connect to the cluster'; sleep 120;
done;count=0;
until $ADMIN -cacert /certs/ca.crt -cert /certs/tls.crt -key /certs/tls.key -cd /usr/share/opensearch/config/opensearch-security -icl -nhnv -h no-securityconfig-tls-configured.no-securityconfig-tls-configured.svc.cluster.local -p 9200 || (( count++ >= 20 ));
do
sleep 20;
done;`

			Expect(createdJob.Spec.Template.Spec.Containers[0].Args[0]).To(Equal(cmdArg))
		})
	})
})
