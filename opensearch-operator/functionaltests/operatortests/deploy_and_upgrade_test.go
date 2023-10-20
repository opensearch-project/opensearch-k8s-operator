package operatortests

import (
	"context"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DeployAndUpgrade", Ordered, func() {
	name := "deploy-and-upgrade"
	namespace := "default"

	BeforeAll(func() {
		CreateKubernetesObjects(name)
	})

	When("creating a cluster", Ordered, func() {
		It("should have 3 ready master pods", func() {
			sts := appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-masters", Namespace: namespace}, &sts)
				if err == nil {
					return sts.Status.ReadyReplicas
				}
				return 0
			}, time.Minute*15, time.Second*5).Should(Equal(int32(3)))
		})

		It("should have a ready dashboards pod", func() {
			deployment := appsv1.Deployment{}
			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-dashboards", Namespace: namespace}, &deployment)
				if err == nil {
					return deployment.Status.ReadyReplicas
				}
				return 0
			}, time.Minute*3, time.Second*5).Should(Equal(int32(1)))
		})
	})

	When("Upgrading the cluster", Ordered, func() {
		It("should accept the version upgrade", func() {
			cluster := unstructured.Unstructured{}
			cluster.SetGroupVersionKind(schema.GroupVersionKind{Group: "opensearch.opster.io", Version: "v1", Kind: "OpenSearchCluster"})
			Get(&cluster, client.ObjectKey{Name: name, Namespace: namespace}, time.Second*5)

			SetNestedKey(cluster.Object, "2.3.0", "spec", "general", "version")
			SetNestedKey(cluster.Object, "2.3.0", "spec", "dashboards", "version")

			Expect(k8sClient.Update(context.Background(), &cluster)).ToNot(HaveOccurred())
		})

		It("should upgrade master pods", func() {
			sts := appsv1.StatefulSet{}
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-masters", Namespace: namespace}, &sts)
				if err == nil {
					return sts.Spec.Template.Spec.Containers[0].Image
				}
				return ""
			}, time.Minute*1, time.Second*5).Should(Equal("docker.io/opensearchproject/opensearch:2.3.0"))

			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-masters", Namespace: namespace}, &sts)
				if err == nil {
					GinkgoWriter.Printf("%+v\n", sts.Status)
					pods := &corev1.PodList{}
					err := k8sClient.List(context.Background(), pods, client.InNamespace(namespace))
					if err == nil {
						for _, pod := range pods.Items {
							revision, ok := pod.Labels["controller-revision-hash"]
							GinkgoWriter.Printf("Pod: %s\tPhase: %s", pod.Name, pod.Status.Phase)
							if ok {
								GinkgoWriter.Printf("\tRevision: %s\t Image: %s", revision, pod.Spec.Containers[0].Image)
							}
							GinkgoWriter.Println()
						}
					} else {
						GinkgoWriter.Println(err)
					}
					cluster := &opsterv1.OpenSearchCluster{}
					k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, cluster)
					GinkgoWriter.Printf("Cluster: %+v\n", cluster.Status)

					return sts.Status.UpdatedReplicas
				}
				GinkgoWriter.Println(err)
				return 0
			}, time.Minute*15, time.Second*5).Should(Equal(int32(3)))
		})

		It("should upgrade the dashboard pod", func() {
			deployment := appsv1.Deployment{}
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-dashboards", Namespace: namespace}, &deployment)
				if err == nil {
					return deployment.Spec.Template.Spec.Containers[0].Image
				}
				return ""
			}, time.Minute*1, time.Second*5).Should(Equal("docker.io/opensearchproject/opensearch-dashboards:2.3.0"))

			Eventually(func() int32 {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name + "-dashboards", Namespace: namespace}, &deployment)
				if err == nil {
					return deployment.Status.ReadyReplicas
				}
				return 0
			}, time.Minute*8, time.Second*5).Should(Equal(int32(1)))
		})
	})

	AfterAll(func() {
		Cleanup(name)
	})
})
