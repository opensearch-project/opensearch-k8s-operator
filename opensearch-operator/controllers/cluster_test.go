package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	. "github.com/kralicky/kmatch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Cluster Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-cluster"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster      = ComposeOpensearchCrd(clusterName, namespace)
		service                = corev1.Service{}
		preUpgradeStatusLength int
	)

	/// ------- Creation Check phase -------

	When("Creating a OpenSearch CRD instance", func() {
		It("Should create the namespace first", func() {
			Expect(CreateNamespace(k8sClient, &OpensearchCluster)).Should(Succeed())
			By("Create cluster ns ")
			Eventually(func() bool {
				return IsNsCreated(k8sClient, namespace)
			}, timeout, interval).Should(BeTrue())
		})
		It("should create the secret for volumes", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: OpensearchCluster.Namespace,
				},
				StringData: map[string]string{
					"test.yml": "foobar",
				},
			}
			Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())
			Eventually(func() error {
				return k8sClient.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{})
			}, timeout, interval).Should(Succeed())
		})

		It("should create the configmap for volumes", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: OpensearchCluster.Namespace,
				},
				Data: map[string]string{
					"test.yml": "foobar",
				},
			}
			Expect(k8sClient.Create(context.Background(), cm)).To(Succeed())
			Eventually(func() error {
				return k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
			}, timeout, interval).Should(Succeed())
		})

		It("should apply the cluster instance successfully", func() {
			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())

		})

		It("should create a ServiceMonitor for the cluster", func() {
			sm := &monitoring.ServiceMonitor{}
			secret := &corev1.Secret{}
			Eventually(func() error {
				// check if the ServiceMonitor created
				return k8sClient.Get(context.Background(), client.ObjectKey{Name: OpensearchCluster.Name + "-monitor", Namespace: OpensearchCluster.Namespace}, sm)
			}, timeout, interval).Should(Succeed())

			// check if the Auth secret created

			Eventually(func() error {
				return k8sClient.Get(context.Background(), client.ObjectKey{Name: OpensearchCluster.Name + "-admin-password", Namespace: OpensearchCluster.Namespace}, secret)
			}, timeout, interval).Should(Succeed())

			// check if the ServiceMonitor is using the Admin secret for basicAuth

			Expect(sm.Spec.Endpoints[0].BasicAuth).Should(BeEquivalentTo(
				&monitoring.BasicAuth{Username: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: OpensearchCluster.Name + "-admin-password"},
					Key:                  "username"},
					Password: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: OpensearchCluster.Name + "-admin-password"},
						Key:                  "password"},
				}))

			// check if the ServiceMonitor is using the interval from the CRD declaration
			Expect(sm.Spec.Endpoints[0].Interval).Should(BeEquivalentTo(OpensearchCluster.Spec.General.Monitoring.ScrapeInterval))

			// check if the ServiceMonitor is using the tlsConfig.insecureSkipVerify from the CRD declaration
			Expect(sm.Spec.Endpoints[0].TLSConfig.InsecureSkipVerify).Should(BeEquivalentTo(OpensearchCluster.Spec.General.Monitoring.TLSConfig.InsecureSkipVerify))

			// check if the ServiceMonitor is using the tlsConfig.serverName from the CRD declaration
			Expect(sm.Spec.Endpoints[0].TLSConfig.ServerName).Should(BeEquivalentTo(OpensearchCluster.Spec.General.Monitoring.TLSConfig.ServerName))

			// check if tlsConfig is not defined in the CRD declaration the ServiceMonitor not deploy that part of the config
			// Expect(sm.Spec.Endpoints[0].TLSConfig).To(BeNil())

		})
	})

	/// ------- Tests logic Check phase -------

	When("Creating a OpenSearchCluster kind Instance", func() {
		It("should create a new opensearch cluster ", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: OpensearchCluster.Spec.General.ServiceName}, &service); err != nil {
					return false
				}
				for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
					nodePool := appsv1.StatefulSet{}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: fmt.Sprintf("%s-%s", OpensearchCluster.Spec.General.ServiceName, nodePoolSpec.Component)}, &service); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("should configure statefulsets correctly", func() {
			wg := sync.WaitGroup{}
			for _, nodePool := range OpensearchCluster.Spec.NodePools {
				wg.Add(1)
				By(fmt.Sprintf("checking %s nodepool", nodePool.Component))
				go func(nodePool opsterv1.NodePool) {
					defer GinkgoRecover()
					defer wg.Done()
					Eventually(Object(&appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      clusterName + "-" + nodePool.Component,
							Namespace: OpensearchCluster.Namespace,
						},
					}, k8sClient), timeout, interval).Should(ExistAnd(
						HaveMatchingContainer(And(
							HaveImage("docker.io/opensearchproject/opensearch:2.0.0"),
							HaveLimits(corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							}),
							HaveEnv("foo", "bar"),
							HaveVolumeMounts(
								"test-secret",
								"test-cm",
								"test-emptydir",
							),
						)),
						HaveMatchingVolume(And(
							HaveName("test-secret"),
							HaveVolumeSource("Secret"),
						)),
						HaveMatchingVolume(And(
							HaveName("test-cm"),
							HaveVolumeSource("ConfigMap"),
						)),
						HaveMatchingVolume(And(
							HaveName("test-emptydir"),
							HaveVolumeSource("emptyDir"),
						)),
					))
				}(nodePool)
			}
			wg.Wait()
		})

		It("should set nodepool specific config", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-client", OpensearchCluster.Name),
					Namespace: OpensearchCluster.Namespace,
				}, sts)
			}, timeout, interval).Should(Succeed())
			Expect(sts.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "baz",
				Value: "bat",
			}))
		})

		It("should set nodepool additional user defined env vars", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-client", OpensearchCluster.Name),
					Namespace: OpensearchCluster.Namespace,
				}, sts)
			}, timeout, interval).Should(Succeed())
			// Based on key/value
			Expect(sts.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "qux",
				Value: "qut",
			}))
			// Based on key/fieldRef of user defined label
			Expect(sts.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "quuxe",
				ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.labels['quux']"}},
			}))
		})

		It("should set nodepool additional user defined labels", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-client", OpensearchCluster.Name),
					Namespace: OpensearchCluster.Namespace,
				}, sts)
			}, timeout, interval).Should(Succeed())
			Expect(sts.ObjectMeta.Labels).To(HaveKeyWithValue("quux", "quut"))
		})

		It("should set nodepool topologySpreadConstraints", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      fmt.Sprintf("%s-master", OpensearchCluster.Name),
					Namespace: OpensearchCluster.Namespace,
				}, sts)
			}, timeout, interval).Should(Succeed())
			Expect(sts.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).To(Equal("zone"))
		})

		It("should create a bootstrap pod", func() {
			bootstrapName := fmt.Sprintf("%s-bootstrap-0", OpensearchCluster.Name)
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      bootstrapName,
					Namespace: OpensearchCluster.Namespace,
				}, &corev1.Pod{})
			}, timeout, interval).Should(Succeed())
			wg := sync.WaitGroup{}
			for _, nodePool := range OpensearchCluster.Spec.NodePools {
				wg.Add(1)
				By(fmt.Sprintf("checking %s nodepool initial master", nodePool.Component))
				go func(nodePool opsterv1.NodePool) {
					defer GinkgoRecover()
					defer wg.Done()
					Eventually(func() []corev1.EnvVar {
						sts := &appsv1.StatefulSet{}
						if err := k8sClient.Get(context.Background(), types.NamespacedName{
							Namespace: OpensearchCluster.Namespace,
							Name:      clusterName + "-" + nodePool.Component,
						}, sts); err != nil {
							return []corev1.EnvVar{}
						}
						return sts.Spec.Template.Spec.Containers[0].Env
					}, timeout, interval).Should(ContainElement(corev1.EnvVar{
						Name:  "cluster.initial_master_nodes",
						Value: bootstrapName,
					}))
				}(nodePool)
			}
			wg.Wait()
		})
		It("should configure bootstrap pod correctly", func() {
			bootstrapName := fmt.Sprintf("%s-bootstrap-0", OpensearchCluster.Name)
			pod := &corev1.Pod{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      bootstrapName,
					Namespace: OpensearchCluster.Namespace,
				}, pod)
			}, timeout, interval).Should(Succeed())
			Expect(pod.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("125m"))
			Expect(pod.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("1Gi"))
			Expect(pod.Spec.Tolerations).To(ContainElement(corev1.Toleration{
				Effect:   "NoSchedule",
				Key:      "foo",
				Operator: "Equal",
				Value:    "bar",
			}))
		})
		It("should create a discovery service", func() {
			discoveryName := fmt.Sprintf("%s-discovery", OpensearchCluster.Name)
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      discoveryName,
					Namespace: OpensearchCluster.Namespace,
				}, &corev1.Service{})
			}, timeout, interval).Should(Succeed())
			wg := sync.WaitGroup{}
			for _, nodePool := range OpensearchCluster.Spec.NodePools {
				wg.Add(1)
				By(fmt.Sprintf("checking %s nodepool initial master", nodePool.Component))
				go func(nodePool opsterv1.NodePool) {
					defer GinkgoRecover()
					defer wg.Done()
					Eventually(func() []corev1.EnvVar {
						sts := &appsv1.StatefulSet{}
						if err := k8sClient.Get(context.Background(), types.NamespacedName{
							Namespace: OpensearchCluster.Namespace,
							Name:      clusterName + "-" + nodePool.Component,
						}, sts); err != nil {
							return []corev1.EnvVar{}
						}
						return sts.Spec.Template.Spec.Containers[0].Env
					}, timeout, interval).Should(ContainElement(corev1.EnvVar{
						Name:  "discovery.seed_hosts",
						Value: discoveryName,
					}))
				}(nodePool)
			}
			wg.Wait()
		})
		It("should set correct owner references", func() {
			service := corev1.Service{}
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName}, &service)).To(Succeed())
			Expect(HasOwnerReference(&service, &OpensearchCluster)).To(BeTrue())
			for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
				nodePool := appsv1.StatefulSet{}
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool)).To(Succeed())
				Expect(HasOwnerReference(&nodePool, &OpensearchCluster)).To(BeTrue())
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-" + nodePoolSpec.Component}, &service)).To(Succeed())
				Expect(HasOwnerReference(&service, &OpensearchCluster)).To(BeTrue())
			}
		})
		It("should set the version status", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster); err != nil {
					return false
				}
				return OpensearchCluster.Status.Version == "2.0.0"
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Tests nodepool cleanup -------
	When("Updating an OpensearchCluster kind instance", func() {
		It("should remove old node pools", func() {
			// Fetch the latest version of the opensearch object
			Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster)).Should(Succeed())

			// Update the opensearch object
			OpensearchCluster.Spec.NodePools = OpensearchCluster.Spec.NodePools[:2]
			OpensearchCluster.Spec.General.Version = "1.1.0"
			OpensearchCluster.Spec.General.PluginsList[0] = "http://foo-plugin-1.1.0"
			Expect(k8sClient.Update(context.Background(), &OpensearchCluster)).Should(Succeed())

			Eventually(func() bool {
				stsList := &appsv1.StatefulSetList{}
				err := k8sClient.List(context.Background(), stsList, client.InNamespace(OpensearchCluster.Name))
				if err != nil {
					return false
				}

				return len(stsList.Items) == 2
			})
		})
		It("should update the node pool image version", func() {
			for _, pool := range OpensearchCluster.Spec.NodePools {
				Eventually(func() bool {
					sts := &appsv1.StatefulSet{}
					err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: OpensearchCluster.Namespace, Name: clusterName + "-" + pool.Component}, sts)
					if err != nil {
						return false
					}
					return sts.Spec.Template.Spec.Containers[0].Image == "docker.io/opensearchproject/opensearch:1.1.0"
				}).Should(BeTrue())
			}
		})
	})

	When("A node pool is upgrading", func() {
		Specify("updating the status should succeed", func() {
			status := opsterv1.ComponentStatus{
				Component:   "Upgrader",
				Description: "nodes",
				Status:      "Upgrading",
			}
			Eventually(func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster); err != nil {
					return err
				}
				preUpgradeStatusLength = len(OpensearchCluster.Status.ComponentsStatus)
				OpensearchCluster.Status.ComponentsStatus = append(OpensearchCluster.Status.ComponentsStatus, status)
				return k8sClient.Status().Update(context.Background(), &OpensearchCluster)
			}()).Should(Succeed())
		})
		It("should update the node pool image", func() {
			Eventually(func() bool {
				sts := &appsv1.StatefulSet{}
				if err := k8sClient.Get(
					context.Background(),
					client.ObjectKey{
						Namespace: OpensearchCluster.Namespace,
						Name:      clusterName + "-nodes",
					}, sts); err != nil {
					return false
				}
				return sts.Spec.Template.Spec.Containers[0].Image == "docker.io/opensearchproject/opensearch:1.1.0"
			}, timeout, interval).Should(BeTrue())
		})
		It("should update any plugin URLs", func() {
			Eventually(func() bool {
				sts := &appsv1.StatefulSet{}
				if err := k8sClient.Get(
					context.Background(),
					client.ObjectKey{
						Namespace: OpensearchCluster.Namespace,
						Name:      clusterName + "-nodes",
					}, sts); err != nil {
					return false
				}
				return ArrayElementContains(sts.Spec.Template.Spec.Containers[0].Command, "http://foo-plugin-1.1.0")
			}, timeout, interval).Should(BeTrue())
		})
	})
	When("a cluster is upgraded", func() {
		Specify("updating the status should succeed", func() {
			currentStatus := opsterv1.ComponentStatus{
				Component:   "Upgrader",
				Status:      "Upgrading",
				Description: "nodes",
			}
			componentStatus := opsterv1.ComponentStatus{
				Component:   "Upgrader",
				Status:      "Upgraded",
				Description: "nodes",
			}
			masterComponentStatus := opsterv1.ComponentStatus{
				Component:   "Upgrader",
				Status:      "Upgraded",
				Description: "master",
			}
			Eventually(func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster); err != nil {
					return err
				}
				OpensearchCluster.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, OpensearchCluster.Status.ComponentsStatus)
				OpensearchCluster.Status.ComponentsStatus = append(OpensearchCluster.Status.ComponentsStatus, masterComponentStatus)
				return k8sClient.Status().Update(context.Background(), &OpensearchCluster)
			}, timeout, interval).Should(BeNil())
		})
		It("should cleanup the status", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster); err != nil {
					return false
				}
				return len(OpensearchCluster.Status.ComponentsStatus) == preUpgradeStatusLength
			}, timeout, interval)
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&OpensearchCluster), &OpensearchCluster); err != nil {
					return false
				}
				return OpensearchCluster.Status.Version == "1.1.0"
			}, timeout, interval)
		})
		It("should update all the node pools", func() {
			wg := sync.WaitGroup{}
			for _, nodePool := range OpensearchCluster.Spec.NodePools {
				wg.Add(1)
				go func(nodePool opsterv1.NodePool) {
					defer GinkgoRecover()
					defer wg.Done()
					Eventually(func() bool {
						sts := &appsv1.StatefulSet{}
						if err := k8sClient.Get(context.Background(), types.NamespacedName{
							Namespace: OpensearchCluster.Namespace,
							Name:      clusterName + "-" + nodePool.Component,
						}, sts); err != nil {
							return false
						}
						return sts.Spec.Template.Spec.Containers[0].Image == "docker.io/opensearchproject/opensearch:1.1.0"
					}, timeout, interval).Should(BeTrue())
				}(nodePool)
			}
			wg.Wait()
		})
	})
})
