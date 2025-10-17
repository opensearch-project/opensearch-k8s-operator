package reconcilers

import (
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Bootstrap Pod Reconciliation Fix", func() {
	Context("Bootstrap Pod Recreation Approach", func() {
		It("should detect when any bootstrap pod spec field has changed", func() {
			instance := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "recreation-test",
					Namespace: "test-namespace",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						HttpPort:       9200,
						ServiceName:    "recreation-test",
						Version:        "2.8.0",
						ServiceAccount: "default-sa",
					},
					Bootstrap: opsterv1.BootstrapConfig{
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
				Status: opsterv1.ClusterStatus{
					Initialized: false,
				},
			}

			volumes := []corev1.Volume{}
			volumeMounts := []corev1.VolumeMount{}

			originalPod := builders.NewBootstrapPod(instance, volumes, volumeMounts)

			By("Testing PodSpecChanged utility function")

			// Test 1: Same spec should not trigger recreation
			Expect(util.PodSpecChanged(originalPod, originalPod)).To(BeFalse())

			// Test 2: Different ServiceAccountName should trigger recreation
			modifiedPod := originalPod.DeepCopy()
			modifiedPod.Spec.ServiceAccountName = "new-sa"
			Expect(util.PodSpecChanged(originalPod, modifiedPod)).To(BeTrue())

			// Test 3: Different Tolerations should trigger recreation
			modifiedPod = originalPod.DeepCopy()
			modifiedPod.Spec.Tolerations = []corev1.Toleration{
				{
					Key:      "new-purpose",
					Operator: "Equal",
					Value:    "monitoring",
					Effect:   "NoSchedule",
				},
			}
			Expect(util.PodSpecChanged(originalPod, modifiedPod)).To(BeTrue())

			// Test 4: Different NodeSelector should trigger recreation
			modifiedPod = originalPod.DeepCopy()
			modifiedPod.Spec.NodeSelector = map[string]string{
				"node-type": "compute",
			}
			Expect(util.PodSpecChanged(originalPod, modifiedPod)).To(BeTrue())

			// Test 5: Different environment variables should trigger recreation
			modifiedPod = originalPod.DeepCopy()
			if len(modifiedPod.Spec.Containers) > 0 {
				modifiedPod.Spec.Containers[0].Env = append(modifiedPod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "NEW_VAR",
					Value: "new_value",
				})
			}
			Expect(util.PodSpecChanged(originalPod, modifiedPod)).To(BeTrue())

			// Test 6: Different container image should trigger recreation
			modifiedPod = originalPod.DeepCopy()
			if len(modifiedPod.Spec.Containers) > 0 {
				modifiedPod.Spec.Containers[0].Image = "opensearch:2.9.0"
			}
			Expect(util.PodSpecChanged(originalPod, modifiedPod)).To(BeTrue())

			// Test 7: Different volumes should trigger recreation
			modifiedPod = originalPod.DeepCopy()
			modifiedPod.Spec.Volumes = append(modifiedPod.Spec.Volumes, corev1.Volume{
				Name: "extra-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})
			Expect(util.PodSpecChanged(originalPod, modifiedPod)).To(BeTrue())
		})
	})
})
