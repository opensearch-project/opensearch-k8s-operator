package reconcilers

import (
	"context"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"opensearch.opster.io/pkg/reconcilers/util"

	. "github.com/kralicky/kmatch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo..

func newDashboardsReconciler(spec *opsterv1.OpenSearchCluster) (ReconcilerContext, *DashboardsReconciler) {
	reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, spec.Spec.NodePools)
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

	When("running the dashboards reconciler with TLS enabled and an existing cert in a single secret", func() {
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
							Enable:               true,
							Generate:             false,
							TlsCertificateConfig: opsterv1.TlsCertificateConfig{Secret: corev1.LocalObjectReference{Name: secretName}},
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

	When("running the dashboards reconciler with TLS enabled and generate enabled", func() {
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

	When("running the dashboards reconciler with a credentials secret supplied", func() {
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

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
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

			Eventually(Object(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName + "-dashboards",
					Namespace: clusterName,
				},
			}, k8sClient), timeout, interval).Should(ExistAnd(
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
			))
		})
	})

	When("running the dashboards reconciler with additionalConfig supplied", func() {
		It("should populate the dashboard config with these values", func() {
			clusterName := "dashboards-add-config"
			testConfig := "some-config-here"

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
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
			underTest := NewDashboardsReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err = underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			configMap := corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-dashboards-config", Namespace: clusterName}, &configMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			data, exists := configMap.Data[helpers.DashboardConfigName]
			Expect(exists).To(BeTrue())
			Expect(strings.Contains(data, testConfig)).To(BeTrue())

			deployment := appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{
					Name:      clusterName + "-dashboards",
					Namespace: clusterName,
				}, &deployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			expectedChecksum, _ := util.GetSha1Sum([]byte(data))
			Expect(deployment.Spec.Template.ObjectMeta.Annotations[helpers.DashboardChecksumName]).To(Equal(expectedChecksum))
		})
	})

	When("running the dashboards reconciler with envs supplied", func() {
		It("should populate the dashboard env vars", func() {
			clusterName := "dashboards-add-env"
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
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
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
			Eventually(Object(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName + "-dashboards",
					Namespace: clusterName,
				},
			}, k8sClient), timeout, interval).Should(ExistAnd(
				HaveMatchingContainer(
					HaveEnv(
						"TEST",
						"TEST",
					),
				),
			))
		})
	})

	When("running the dashboards reconciler with optional image spec supplied", func() {
		It("should populate the dashboard image specification with these values", func() {
			clusterName := "dashboards-add-image-spec"
			image := "docker.io/my-opensearch-dashboards:custom"
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
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.Background(), &ns)
			Expect(err).ToNot(HaveOccurred())

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)
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

			actualImage := deployment.Spec.Template.Spec.Containers[0].Image
			actualImagePullPolicy := deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy
			Expect(actualImage).To(Equal(image))
			Expect(actualImagePullPolicy).To(Equal(imagePullPolicy))
		})
	})

	When("running the dashboards reconciler with extra volumes", func() {
		clusterName := "dashboards-add-volumes"
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
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: ns.Name,
			},
			StringData: map[string]string{
				"test.yml": "foobar",
			},
		}
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: ns.Name,
			},
			Data: map[string]string{
				"test.yml": "foobar",
			},
		}
		Context("set up the dashboards", func() {
			It("should create the namespace", func() {
				Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(context.Background(), client.ObjectKeyFromObject(ns), &corev1.Namespace{})
				}, timeout, interval).Should(Succeed())
			})
			It("should create the secret for volumes", func() {
				Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{})
				}, timeout, interval).Should(Succeed())
			})

			It("should create the configmap for volumes", func() {
				Expect(k8sClient.Create(context.Background(), cm)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
				}, timeout, interval).Should(Succeed())
			})
		})
		It("mount the volumes in the deployment", func() {
			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, spec.Spec.NodePools)
			underTest := NewDashboardsReconciler(
				k8sClient,
				context.Background(),
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				spec,
			)
			Expect(func() error {
				_, err := underTest.Reconcile()
				return err
			}()).To(Succeed())

			Eventually(Object(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName + "-dashboards",
					Namespace: clusterName,
				},
			}, k8sClient), timeout, interval).Should(ExistAnd(
				HaveMatchingContainer(
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
