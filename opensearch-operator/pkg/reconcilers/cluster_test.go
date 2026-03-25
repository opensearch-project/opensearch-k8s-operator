package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Bootstrap Pod Hash-Based Reconciliation", func() {
	Context("Hash-based change detection", func() {
		var instance *opensearchv1.OpenSearchCluster

		BeforeEach(func() {
			instance = &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hash-test",
					Namespace: "test-namespace",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						HttpPort:       9200,
						ServiceName:    "hash-test",
						Version:        "2.8.0",
						ServiceAccount: "default-sa",
					},
					Bootstrap: opensearchv1.BootstrapConfig{
						Tolerations: []corev1.Toleration{
							{
								Key:      "purpose",
								Operator: "Equal",
								Value:    "logging",
								Effect:   "NoSchedule",
							},
						},
					},
				},
			}
		})

		It("should produce the same hash for the same CR", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)
			pod2 := builders.NewBootstrapPod(instance, nil, nil)

			hash1 := pod1.Annotations[builders.BootstrapPodSpecHashAnnotation]
			hash2 := pod2.Annotations[builders.BootstrapPodSpecHashAnnotation]

			Expect(hash1).NotTo(BeEmpty())
			Expect(hash1).To(Equal(hash2))
		})

		It("should produce a different hash when the CR image changes", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)

			modified := instance.DeepCopy()
			customImage := "opensearch:2.9.0"
			modified.Spec.General.ImageSpec = &opensearchv1.ImageSpec{
				Image: &customImage,
			}
			pod2 := builders.NewBootstrapPod(modified, nil, nil)

			Expect(pod1.Annotations[builders.BootstrapPodSpecHashAnnotation]).
				NotTo(Equal(pod2.Annotations[builders.BootstrapPodSpecHashAnnotation]))
		})

		It("should produce a different hash when tolerations change", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)

			modified := instance.DeepCopy()
			modified.Spec.Bootstrap.Tolerations = []corev1.Toleration{
				{
					Key:      "new-purpose",
					Operator: "Equal",
					Value:    "monitoring",
					Effect:   "NoSchedule",
				},
			}
			pod2 := builders.NewBootstrapPod(modified, nil, nil)

			Expect(pod1.Annotations[builders.BootstrapPodSpecHashAnnotation]).
				NotTo(Equal(pod2.Annotations[builders.BootstrapPodSpecHashAnnotation]))
		})

		It("should produce a different hash when service account changes", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)

			modified := instance.DeepCopy()
			modified.Spec.General.ServiceAccount = "new-sa"
			pod2 := builders.NewBootstrapPod(modified, nil, nil)

			Expect(pod1.Annotations[builders.BootstrapPodSpecHashAnnotation]).
				NotTo(Equal(pod2.Annotations[builders.BootstrapPodSpecHashAnnotation]))
		})

		It("should set the hash annotation on the built pod", func() {
			pod := builders.NewBootstrapPod(instance, nil, nil)

			Expect(pod.Annotations).To(HaveKey(builders.BootstrapPodSpecHashAnnotation))
			Expect(pod.Annotations[builders.BootstrapPodSpecHashAnnotation]).To(HaveLen(40)) // SHA1 hex digest
		})
	})
})
