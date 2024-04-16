package controllers

import (
	"context"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Scaler Reconciler", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-nodes"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(clusterName, namespace)
		nodePool          = appsv1.StatefulSet{}
		cluster2          = opsterv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	Context("When create OpenSearch CRD - nodes", func() {
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
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				}, &corev1.Secret{})
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
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      cm.Name,
					Namespace: cm.Namespace,
				}, &corev1.ConfigMap{})
			}, timeout, interval).Should(Succeed())
		})

		It("should apply the cluster instance successfully", func() {
			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("When changing Opensearch NodePool Replicas", func() {
		It("should add a new status about the operation", func() {
			By("Wait for cluster instance to be created")
			Eventually(func() bool {
				return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &OpensearchCluster) == nil
			}, time.Second*10, interval).Should(BeTrue())
			By("Update replicas")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &OpensearchCluster); err != nil {
					return err
				}
				OpensearchCluster.Spec.NodePools[0].Replicas = 2

				return k8sClient.Update(context.Background(), &OpensearchCluster)
			})
			Expect(err).ToNot(HaveOccurred())
			status := len(OpensearchCluster.Status.ComponentsStatus)

			By("Check ComponentsStatus")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &cluster2); err != nil {
					return false
				}
				return status != len(cluster2.Status.ComponentsStatus)
			}, time.Second*60, 30*time.Millisecond).Should(BeFalse())
		})
	})

	Context("When changing CRD nodepool replicas", func() {
		It("should implement new number of replicas to the cluster", func() {
			By("check replicas")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: clusterName + "-" + cluster2.Spec.NodePools[0].Component}, &nodePool); err != nil {
					return false
				}
				if pointer.Int32Deref(nodePool.Spec.Replicas, 1) != 2 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	//// ------- Tests logic Check phase for scaling DiskSize -------

	Context("When changing Opensearch NodePool DiskSize", func() {
		It("should add a new status about the operation", func() {
			By("Wait for cluster instance to be created")
			Eventually(func() bool {
				return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &OpensearchCluster) == nil
			}, time.Second*10, interval).Should(BeTrue())
			By("Update diskSize")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &OpensearchCluster); err != nil {
					return err
				}
				if OpensearchCluster.Spec.NodePools[0].Persistence == nil || OpensearchCluster.Spec.NodePools[0].Persistence.PersistenceSource.PVC != nil {
					OpensearchCluster.Spec.NodePools[0].DiskSize = "32Gi"
				}

				return k8sClient.Update(context.Background(), &OpensearchCluster)
			})
			Expect(err).ToNot(HaveOccurred())
			status := len(OpensearchCluster.Status.ComponentsStatus)

			By("Check ComponentsStatus")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpensearchCluster.Name}, &cluster2); err != nil {
					return false
				}
				return status != len(cluster2.Status.ComponentsStatus)
			}, time.Second*60, 30*time.Millisecond).Should(BeFalse())
		})
	})

	Context("When changing CRD nodepool DiskSize", func() {
		It("should implement new DiskSize to the cluster", func() {
			By("check diskSize")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: clusterName + "-" + cluster2.Spec.NodePools[0].Component}, &nodePool); err != nil {
					return false
				}
				if OpensearchCluster.Spec.NodePools[0].Persistence == nil || OpensearchCluster.Spec.NodePools[0].Persistence.PersistenceSource.PVC != nil {
					existingDisk := nodePool.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String()
					return existingDisk == "32Gi"
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Deletion Check phase -------

	Context("When deleting OpenSearch CRD ", func() {
		It("should set correct owner references", func() {
			for _, nodePoolSpec := range OpensearchCluster.Spec.NodePools {
				nodePool := appsv1.StatefulSet{}
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool)).To(Succeed())
				Expect(HasOwnerReference(&nodePool, &OpensearchCluster)).To(BeTrue())
			}
		})
	})
})
