package reconcilers

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func newDashboardsReconciler(spec *opsterv1.OpenSearchCluster) (ReconcilerContext, *DashboardsReconciler) {
	reconcilerContext := NewReconcilerContext()
	underTest := NewDashboardsReconciler(
		k8sClient,
		context.Background(),
		&helpers.MockEventRecorder{},
		&reconcilerContext,
		spec,
	)
	underTest.pki = helpers.NewMockPKI()
	return reconcilerContext, underTest
}

var _ = Describe("Dashboards Reconciler", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	Context("When running the dashboards reconciler with TLS enabled and an existing cert in a single secret", func() {
		It("should mount the secret", func() {
			clusterName := "dashboards-singlesecret"
			secretName := "my-cert"

			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Tls: &opsterv1.DashboardsTlsConfig{
							Enable:            true,
							Generate:          false,
							CertificateConfig: opsterv1.TlsCertificateConfig{Secret: corev1.LocalObjectReference{Name: secretName}},
						},
					},
				}}

			_, underTest := newDashboardsReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			deployment := appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-dashboards", Namespace: clusterName}, &deployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(helpers.CheckVolumeExists(deployment.Spec.Template.Spec.Volumes, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, secretName, "tls-cert")).Should((BeTrue()))
		})
	})

	Context("When running the dashboards reconciler with TLS enabled and generate enabled", func() {
		It("should create a cert", func() {
			clusterName := "dashboards-test-generate"
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
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())
			_, underTest := newDashboardsReconciler(&spec)
			underTest.pki = helpers.NewMockPKI()
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			// Check if secret is mounted
			deployment := appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-dashboards", Namespace: clusterName}, &deployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(helpers.CheckVolumeExists(deployment.Spec.Template.Spec.Volumes, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, clusterName+"-dashboards-cert", "tls-cert")).Should((BeTrue()))
			// Check if secret contains correct data keys
			secret := corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-dashboards-cert", Namespace: clusterName}, &secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(helpers.HasKeyWithBytes(secret.Data, "tls.key")).To(BeTrue())
			Expect(helpers.HasKeyWithBytes(secret.Data, "tls.crt")).To(BeTrue())
		})
	})

	Context("When running the dashboards reconciler with a credentials secret supplied", func() {
		It("should provide these credentials as env vars", func() {
			clusterName := "dashboards-creds"
			credentialsSecret := clusterName + "-creds"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable:                      true,
						OpensearchCredentialsSecret: corev1.LocalObjectReference{Name: credentialsSecret},
					},
				}}
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())

			reconcilerContext := NewReconcilerContext()
			underTest := NewDashboardsReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			deployment := appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-dashboards", Namespace: clusterName}, &deployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			env := deployment.Spec.Template.Spec.Containers[0].Env
			Expect(hasEnvWithSecretSource(env, "OPENSEARCH_USERNAME", credentialsSecret, "username")).To(BeTrue())
			Expect(hasEnvWithSecretSource(env, "OPENSEARCH_PASSWORD", credentialsSecret, "password")).To(BeTrue())
		})
	})

})

func hasEnvWithSecretSource(env []corev1.EnvVar, name string, secretName string, secretKey string) bool {
	for _, envVar := range env {
		if envVar.Name != name {
			continue
		}
		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil {
			return false
		}
		return envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name == secretName && envVar.ValueFrom.SecretKeyRef.Key == secretKey
	}
	return false
}
