package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TLS Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	Context("When Reconciling the TLS configuration with no existing secrets", func() {
		It("should create the needed secrets ", func() {
			clusterName := "tls-test"
			caSecretName := "tls-test-ca"
			transportSecretName := "tls-test-transport-cert"
			httpSecretName := "tls-test-http-cert"
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{ClusterName: clusterName},
				Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
					Transport: &opsterv1.TlsConfigTransport{Generate: true},
					Http:      &opsterv1.TlsConfigHttp{Generate: true},
				}},
			}}
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(controllerContext.Volumes).Should(HaveLen(2))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(2))
			//fmt.Printf("%s\n", controllerContext.OpenSearchConfig)
			value, exists := controllerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=tls-test,OU=tls-test\"]"))

			Eventually(func() bool {
				caSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &caSecret)
				if err != nil {
					return false
				}
				transportSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: transportSecretName, Namespace: clusterName}, &transportSecret)
				if err != nil {
					return false
				}
				httpSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: httpSecretName, Namespace: clusterName}, &httpSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("When Reconciling the TLS configuration with no existing secrets and perNode certs activated", func() {
		It("should create the needed secrets ", func() {
			clusterName := "tls-pernode"
			caSecretName := "tls-pernode-ca"
			transportSecretName := "tls-pernode-transport-cert"
			httpSecretName := "tls-pernode-http-cert"
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{ClusterName: clusterName},
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
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}
			err := k8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())

			Expect(controllerContext.Volumes).Should(HaveLen(2))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(2))

			value, exists := controllerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=*,OU=tls-pernode\"]"))

			Eventually(func() bool {
				caSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: caSecretName, Namespace: clusterName}, &caSecret)
				if err != nil {
					return false
				}
				transportSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: transportSecretName, Namespace: clusterName}, &transportSecret)
				if err != nil {
					return false
				}
				if _, exists := transportSecret.Data["ca.crt"]; !exists {
					fmt.Printf("ca.crt missing from transport secret\n")
					return false
				}
				for i := 0; i < 3; i++ {
					name := fmt.Sprintf("tls-pernode-masters-%d", i)
					if _, exists := transportSecret.Data[name+".crt"]; !exists {
						fmt.Printf("%s.crt missing from transport secret\n", name)
						return false
					}
					if _, exists := transportSecret.Data[name+".key"]; !exists {
						fmt.Printf("%s.key missing from transport secret\n", name)
						return false
					}
				}
				httpSecret := corev1.Secret{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: httpSecretName, Namespace: clusterName}, &httpSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("When Reconciling the TLS configuration with external certificates", func() {
		It("Should not create secrets but only mount them", func() {
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: "tls-test-existingsecrets"}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
				Transport: &opsterv1.TlsConfigTransport{
					Generate: false,
					CertificateConfig: opsterv1.TlsCertificateConfig{
						CaSecret:   &opsterv1.TlsSecret{SecretName: "casecret-transport"},
						KeySecret:  &opsterv1.TlsSecret{SecretName: "keysecret-transport"},
						CertSecret: &opsterv1.TlsSecret{SecretName: "certsecret-transport"},
					},
					NodesDn: []string{"CN=mycn", "CN=othercn"},
				},
				Http: &opsterv1.TlsConfigHttp{
					Generate: false,
					CertificateConfig: opsterv1.TlsCertificateConfig{
						CaSecret:   &opsterv1.TlsSecret{SecretName: "casecret-http"},
						KeySecret:  &opsterv1.TlsSecret{SecretName: "keysecret-http"},
						CertSecret: &opsterv1.TlsSecret{SecretName: "certsecret-http"},
					},
				},
			},
			}}}
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tls-test-existingsecrets",
				},
			}
			err := k8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())

			Expect(controllerContext.Volumes).Should(HaveLen(6))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(6))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "casecret-transport", "transport-ca")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "keysecret-transport", "transport-key")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "certsecret-transport", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "casecret-http", "http-ca")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "keysecret-http", "http-key")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "certsecret-http", "http-cert")).Should((BeTrue()))

			value, exists := controllerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
		})
	})

	Context("When Reconciling the TLS configuration with external per-node certificates", func() {
		It("Should not create secrets but only mount them", func() {
			spec := opsterv1.OpenSearchCluster{Spec: opsterv1.ClusterSpec{General: opsterv1.GeneralConfig{ClusterName: "tls-test-existingsecretspernode"}, Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
				Transport: &opsterv1.TlsConfigTransport{
					Generate: false,
					PerNode:  true,
					CertificateConfig: opsterv1.TlsCertificateConfig{
						Secret: "my-transport-certs",
					},
					NodesDn: []string{"CN=mycn", "CN=othercn"},
				},
				Http: &opsterv1.TlsConfigHttp{
					Generate: false,
					CertificateConfig: opsterv1.TlsCertificateConfig{
						Secret: "my-http-certs",
					},
				},
			},
			}}}
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tls-test-existingsecretspernode",
				},
			}
			err := k8sClient.Create(context.TODO(), &ns)
			Expect(err).ToNot(HaveOccurred())
			underTest := TlsReconciler{
				Client:   k8sClient,
				Instance: &spec,
				Logger:   logr.Discard(),
				//Recorder: recorder,
			}
			controllerContext := NewControllerContext()
			_, err = underTest.Reconcile(&controllerContext)
			Expect(err).ToNot(HaveOccurred())
			Expect(controllerContext.Volumes).Should(HaveLen(2))
			Expect(controllerContext.VolumeMounts).Should(HaveLen(2))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "my-transport-certs", "transport-certs")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(controllerContext.Volumes, controllerContext.VolumeMounts, "my-http-certs", "http-certs")).Should((BeTrue()))

			value, exists := controllerContext.OpenSearchConfig["plugins.security.nodes_dn"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("[\"CN=mycn\",\"CN=othercn\"]"))
		})
	})

	Context("When Creating an OpenSearchCluster with TLS configured", func() {
		It("Should start a cluster successfully", func() {
			clusterName := "tls-cluster"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: "default"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ClusterName: clusterName, ServiceName: clusterName},
					Security: &opsterv1.Security{Tls: &opsterv1.TlsConfig{
						Transport: &opsterv1.TlsConfigTransport{
							Generate: true,
							PerNode:  true,
						},
						Http: &opsterv1.TlsConfigHttp{
							Generate: true,
						},
					}},
					NodePools: []opsterv1.NodePool{
						{
							Component: "masters",
							Replicas:  3,
							Roles:     []string{"master", "data"},
						},
					},
				}}
			Expect(k8sClient.Create(context.Background(), &spec)).Should(Succeed())

			By("Checking for Statefulset")
			sts := appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-masters", Namespace: clusterName}, &sts)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(*sts.Spec.Replicas).To(Equal(int32(3)))
			Expect(helpers.CheckVolumeExists(sts.Spec.Template.Spec.Volumes, sts.Spec.Template.Spec.Containers[0].VolumeMounts, clusterName+"-transport-cert", "transport-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(sts.Spec.Template.Spec.Volumes, sts.Spec.Template.Spec.Containers[0].VolumeMounts, clusterName+"-http-cert", "http-cert")).Should((BeTrue()))
			Expect(helpers.CheckVolumeExists(sts.Spec.Template.Spec.Volumes, sts.Spec.Template.Spec.Containers[0].VolumeMounts, clusterName+"-config", "config")).Should((BeTrue()))
			// Cleanup
			Expect(k8sClient.Delete(context.Background(), &spec)).Should(Succeed())
		})
	})

})
