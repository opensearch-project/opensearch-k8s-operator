package reconcilers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Bootstrap Pod Reconciliation", func() {
	Context("Last-applied annotation change detection", func() {
		var instance *opensearchv1.OpenSearchCluster

		BeforeEach(func() {
			instance = &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "last-applied-test",
					Namespace: "test-namespace",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						HttpPort:       9200,
						ServiceName:    "last-applied-test",
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

		It("should produce identical last-applied config for the same CR", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)
			pod2 := builders.NewBootstrapPod(instance, nil, nil)

			cfg1, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod1, false)
			Expect(err).NotTo(HaveOccurred())
			cfg2, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod2, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg1).To(Equal(cfg2))
		})

		It("should produce different last-applied config when image changes", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)

			modified := instance.DeepCopy()
			customImage := "opensearch:2.9.0"
			modified.Spec.General.ImageSpec = &opensearchv1.ImageSpec{
				Image: &customImage,
			}
			pod2 := builders.NewBootstrapPod(modified, nil, nil)

			cfg1, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod1, false)
			Expect(err).NotTo(HaveOccurred())
			cfg2, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod2, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg1).NotTo(Equal(cfg2))
		})

		It("should produce different last-applied config when tolerations change", func() {
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

			cfg1, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod1, false)
			Expect(err).NotTo(HaveOccurred())
			cfg2, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod2, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg1).NotTo(Equal(cfg2))
		})

		It("should produce different last-applied config when service account changes", func() {
			pod1 := builders.NewBootstrapPod(instance, nil, nil)

			modified := instance.DeepCopy()
			modified.Spec.General.ServiceAccount = "new-sa"
			pod2 := builders.NewBootstrapPod(modified, nil, nil)

			cfg1, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod1, false)
			Expect(err).NotTo(HaveOccurred())
			cfg2, err := patch.DefaultAnnotator.GetModifiedConfiguration(pod2, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg1).NotTo(Equal(cfg2))
		})
	})
})
