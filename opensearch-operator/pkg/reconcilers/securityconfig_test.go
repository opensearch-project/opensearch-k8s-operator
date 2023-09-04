package reconcilers

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Securityconfig Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "securityconfig"
		timeout     = time.Second * 10
		interval    = time.Second * 1
	)

	When("When Reconciling the securityconfig reconciler with no securityconfig provided in the spec", func() {
		It("should not do anything ", func() {
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}}}

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
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

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
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
			// Create namespace and secrets first
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())

			securityConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				StringData: map[string]string{
					"config.yml":         "foobar",
					"internal_users.yml": "bar",
					// Invalid yml in secret should not throw an error
					"invalid.yml": "foo",
					// Empty contents for a yml should be ignored
					"action_groups.yml": "",
				},
			}
			err := k8sClient.Create(context.Background(), securityConfigSecret)
			Expect(err).ToNot(HaveOccurred())

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

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			job := batchv1.Job{}
			// Should not throw an error as the update job should exist
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-securityconfig-update", Namespace: clusterName}, &job)
				return err == nil
			}, timeout, interval).Should(BeTrue())

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
			var clusterName = "securityconfig-withadminsecret"
			// Create namespace and secrets first
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())

			adminCertSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "admin-cert", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				Data:       map[string][]byte{},
			}
			err := k8sClient.Create(context.Background(), adminCertSecret)
			Expect(err).ToNot(HaveOccurred())

			securityConfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig-secret", Namespace: clusterName},
				Type:       corev1.SecretType("Opaque"),
				Data:       map[string][]byte{},
			}
			err = k8sClient.Create(context.Background(), securityConfigSecret)
			Expect(err).ToNot(HaveOccurred())

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
				}}

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			job := batchv1.Job{}
			// Should not throw an error as the update job should exist
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-securityconfig-update", Namespace: clusterName}, &job)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	When("When Reconciling the securityconfig reconciler with securityconfig secret but no adminSecret configured", func() {
		It("should not start an update job", func() {
			var clusterName = "securityconfig-noadminsecret"
			// Create namespace and secret first
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			configSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "securityconfig", Namespace: clusterName},
				StringData: map[string]string{"config.yml": "foobar"},
			}
			err := k8sClient.Create(context.Background(), &configSecret)
			Expect(err).ToNot(HaveOccurred())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					Security: &opsterv1.Security{
						Config: &opsterv1.SecurityConfig{
							SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig"},
						},
					},
				}}

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			job := batchv1.Job{}
			// Should throw an error as the update job should not exist
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-securityconfig-update", Namespace: clusterName}, &job)).To(HaveOccurred())

		})
	})

	When("When Reconciling the securityconfig reconciler with no securityconfig secret but tls configured", func() {
		It("should start an update job and apply all yml files", func() {
			var clusterName = "no-securityconfig-tls-configured"
			// Create namespace and secret first
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
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
				}}

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewSecurityconfigReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			job := batchv1.Job{}
			// Should not throw an error as the update job should exist
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-securityconfig-update", Namespace: clusterName}, &job)
				return err == nil
			}, timeout, interval).Should(BeTrue())

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
			Expect(job.Spec.Template.Spec.Containers[0].Args[0]).To(Equal(cmdArg))
		})
	})
})
