package controllers

import (
	"context"
	"time"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Scaler Reconciler", Ordered, func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-nodes"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpenSearchCluster = ComposeOpenSearchCrd(clusterName, namespace)
		nodePool          = appsv1.StatefulSet{}
		cluster2          = opensearchv1.OpenSearchCluster{}
	)

	/// ------- Creation Check phase -------

	Context("When create OpenSearch CRD - nodes", func() {
		It("Should create the namespace first", func() {
			Expect(CreateNamespace(k8sClient, &OpenSearchCluster)).Should(Succeed())
			By("Create cluster ns ")
			Eventually(func() bool {
				return IsNsCreated(k8sClient, namespace)
			}, timeout, interval).Should(BeTrue())
		})

		It("should create the secret for volumes", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: OpenSearchCluster.Namespace,
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
					Namespace: OpenSearchCluster.Namespace,
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
			Expect(k8sClient.Create(context.Background(), &OpenSearchCluster)).Should(Succeed())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("When changing Opensearch NodePool Replicas", func() {
		It("should add a new status about the operation", func() {
			By("Wait for cluster instance to be created")
			Eventually(func() bool {
				return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpenSearchCluster.Name}, &OpenSearchCluster) == nil
			}, time.Second*10, interval).Should(BeTrue())
			By("Update replicas")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpenSearchCluster.Name}, &OpenSearchCluster); err != nil {
					return err
				}
				OpenSearchCluster.Spec.NodePools[0].Replicas = 2

				return k8sClient.Update(context.Background(), &OpenSearchCluster)
			})
			Expect(err).ToNot(HaveOccurred())
			status := len(OpenSearchCluster.Status.ComponentsStatus)

			By("Check ComponentsStatus")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpenSearchCluster.Name}, &cluster2); err != nil {
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
				if ptr.Deref(nodePool.Spec.Replicas, int32(1)) != 2 {
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
				return k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpenSearchCluster.Name}, &OpenSearchCluster) == nil
			}, time.Second*10, interval).Should(BeTrue())
			By("Update diskSize")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpenSearchCluster.Name}, &OpenSearchCluster); err != nil {
					return err
				}
				if OpenSearchCluster.Spec.NodePools[0].Persistence == nil || OpenSearchCluster.Spec.NodePools[0].Persistence.PVC != nil {
					OpenSearchCluster.Spec.NodePools[0].DiskSize = resource.MustParse("32Gi")
				}

				return k8sClient.Update(context.Background(), &OpenSearchCluster)
			})
			Expect(err).ToNot(HaveOccurred())
			status := len(OpenSearchCluster.Status.ComponentsStatus)

			By("Check ComponentsStatus")
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: OpenSearchCluster.Name}, &cluster2); err != nil {
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
				if OpenSearchCluster.Spec.NodePools[0].Persistence == nil || OpenSearchCluster.Spec.NodePools[0].Persistence.PersistenceSource.PVC != nil {
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
			for _, nodePoolSpec := range OpenSearchCluster.Spec.NodePools {
				nodePool := appsv1.StatefulSet{}
				Expect(k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-" + nodePoolSpec.Component}, &nodePool)).To(Succeed())
				Expect(HasOwnerReference(&nodePool, &OpenSearchCluster)).To(BeTrue())
			}
		})
	})
})
