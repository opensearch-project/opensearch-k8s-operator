package reconcilers

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kralicky/kmatch"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Securityconfig Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName         = "securityconfig"
		timeout             = time.Second * 15
		interval            = time.Second * 1
		consistentlyTimeout = time.Second * 5
	)

	When("Reconciling the securityconfig reconciler with no securityconfig provided", func() {
		It("should not do anything ", func() {
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec:       opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{}}}

			reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
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

	When("Reconciling the securityconfig reconciler with securityconfig secret configured but not available", func() {
		It("should trigger a requeue", func() {
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
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

			reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
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

	When("Reconciling the securityconfig reconciler with securityconfig secret configured and available", func() {
		Context("provided secret does not contain all files", func() {
			clusterName := "securityconfig-a"
			Specify("prepare test", func() {
				Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
				configSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "securityconfig", Namespace: clusterName},
					StringData: map[string]string{"config.yml": "foobar"},
				}
				err := k8sClient.Create(context.Background(), &configSecret)
				Expect(err).ToNot(HaveOccurred())
				adminCertSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "admin-cert", Namespace: clusterName},
					StringData: map[string]string{"tls.crt": "foobar"},
				}
				err = k8sClient.Create(context.Background(), &adminCertSecret)
				Expect(err).ToNot(HaveOccurred())
				spec := opsterv1.OpenSearchCluster{
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{},
						Security: &opsterv1.Security{
							Config: &opsterv1.SecurityConfig{
								SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig"},
								AdminSecret:          corev1.LocalObjectReference{Name: "admin-cert"},
							},
						},
					},
				}
				reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
				underTest := NewSecurityconfigReconciler(
					k8sClient,
					context.Background(),
					&helpers.MockEventRecorder{},
					&reconcilerContext,
					&spec,
				)
				_, err = underTest.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
			It("should create a default secret", func() {
				Eventually(func() error {
					return k8sClient.Get(context.Background(), types.NamespacedName{
						Name:      clusterName + "-default-securityconfig",
						Namespace: clusterName,
					}, &corev1.Secret{})
				}, timeout, interval).Should(Succeed())
			})
			It("should create an update job", func() {
				defaultMode := int32(420)
				Eventually(Object(&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterName + "-securityconfig-update",
						Namespace: clusterName,
					},
				}, k8sClient), timeout, interval).Should(ExistAnd(
					HaveMatchingContainer(
						HaveVolumeMounts(
							"defaultsecurityconfig",
							"securityconfig",
						),
					),
					HaveMatchingVolume(And(
						HaveName("defaultsecurityconfig"),
						HaveVolumeSource(corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  clusterName + "-default-securityconfig",
								DefaultMode: &defaultMode,
							},
						}),
					)),
					HaveMatchingVolume(And(
						HaveName("securityconfig"),
						HaveVolumeSource(corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  "securityconfig",
								DefaultMode: &defaultMode,
							},
						}),
					)),
				))
			})
		})
		Context("provided secret contains all files", func() {
			clusterName := "securityconfig-b"
			Specify("prepare test", func() {
				Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
				configSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "securityconfig", Namespace: clusterName},
					StringData: map[string]string{
						"action_groups.yml":  "foobar",
						"audit.yml":          "foobar",
						"config.yml":         "foobar",
						"internal_uesrs.yml": "foobar",
						"nodes_dn.yml":       "foobar",
						"roles_mapping.yml":  "foobar",
						"roles.yml":          "foobar",
						"tenants.yml":        "foobar",
						"whitelist.yml":      "foobar",
					},
				}
				err := k8sClient.Create(context.Background(), &configSecret)
				Expect(err).ToNot(HaveOccurred())
				adminCertSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "admin-cert", Namespace: clusterName},
					StringData: map[string]string{"tls.crt": "foobar"},
				}
				err = k8sClient.Create(context.Background(), &adminCertSecret)
				Expect(err).ToNot(HaveOccurred())
				spec := opsterv1.OpenSearchCluster{
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{},
						Security: &opsterv1.Security{
							Config: &opsterv1.SecurityConfig{
								SecurityconfigSecret: corev1.LocalObjectReference{Name: "securityconfig"},
								AdminSecret:          corev1.LocalObjectReference{Name: "admin-cert"},
							},
						},
					},
				}
				reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
				underTest := NewSecurityconfigReconciler(
					k8sClient,
					context.Background(),
					&helpers.MockEventRecorder{},
					&reconcilerContext,
					&spec,
				)
				_, err = underTest.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
			It("should not create a default secret", func() {
				Consistently(func() bool {
					err := k8sClient.Get(context.Background(), types.NamespacedName{
						Name:      clusterName + "-default-securityconfig",
						Namespace: clusterName,
					}, &corev1.Secret{})
					if err == nil {
						return false
					}
					if k8serrors.IsNotFound(err) {
						return true
					}
					return false
				}, consistentlyTimeout, interval).Should(BeTrue())
			})
			It("should create an update job", func() {
				defaultMode := int32(420)
				Eventually(Object(&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterName + "-securityconfig-update",
						Namespace: clusterName,
					},
				}, k8sClient), timeout, interval).Should(ExistAnd(
					HaveMatchingContainer(
						HaveVolumeMounts(
							"securityconfig",
						),
					),
					Not(HaveMatchingVolume(And(
						HaveName("defaultsecurityconfig"),
						HaveVolumeSource(corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  clusterName + "-default-securityconfig",
								DefaultMode: &defaultMode,
							},
						}),
					))),
					HaveMatchingVolume(And(
						HaveName("securityconfig"),
						HaveVolumeSource(corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  "securityconfig",
								DefaultMode: &defaultMode,
							},
						}),
					)),
				))
			})
		})
	})

	When("Reconciling the securityconfig reconciler with securityconfig secret but no adminSecret configured", func() {
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

			reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
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
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-securityconfig-update", Namespace: clusterName}, &job)).To(HaveOccurred())

		})
	})
})
