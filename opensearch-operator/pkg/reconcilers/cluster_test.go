package reconcilers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("emptyDir recovery", func() {
	DescribeTable("emptyDirDataLossSuspected",
		func(stats emptyDirPodStats, expected bool) {
			Expect(emptyDirDataLossSuspected(stats)).To(Equal(expected))
		},
		Entry("does not trigger when data pods exist but are not ready", emptyDirPodStats{
			existingDataPods:   3,
			totalDataPods:      3,
			existingMasterPods: 3,
			totalMasterPods:    3,
		}, false),
		Entry("does not trigger when masters exist but are not ready", emptyDirPodStats{
			existingDataPods:   3,
			totalDataPods:      3,
			existingMasterPods: 2,
			totalMasterPods:    3,
		}, false),
		Entry("triggers when all data pods are missing", emptyDirPodStats{
			existingDataPods:   0,
			totalDataPods:      3,
			existingMasterPods: 3,
			totalMasterPods:    3,
		}, true),
		Entry("triggers when master quorum pods are missing", emptyDirPodStats{
			existingDataPods:   3,
			totalDataPods:      3,
			existingMasterPods: 1,
			totalMasterPods:    3,
		}, true),
		Entry("does not trigger when there are no data nodes", emptyDirPodStats{
			existingDataPods:   0,
			totalDataPods:      0,
			existingMasterPods: 3,
			totalMasterPods:    3,
		}, false),
	)

	It("parses the first observed timestamp from component status", func() {
		firstObserved := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
		components := []opensearchv1.ComponentStatus{
			{
				Component:   emptyDirRecoveryComponent,
				Status:      emptyDirRecoveryStatusPending,
				Description: firstObserved.Format(time.RFC3339),
			},
		}

		parsed, ok := emptyDirRecoveryFirstObserved(components)
		Expect(ok).To(BeTrue())
		Expect(parsed).To(Equal(firstObserved))
	})
})

var _ = Describe("Bootstrap Pod Reconciliation Fix", func() {
	Context("Bootstrap Pod Recreation Approach", func() {
		It("should detect when any bootstrap pod spec field has changed", func() {
			instance := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "recreation-test",
					Namespace: "test-namespace",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						HttpPort:       9200,
						ServiceName:    "recreation-test",
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
				Status: opensearchv1.ClusterStatus{
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

			// Test 8: NodeName changes set by the scheduler should be ignored
			modifiedPod = originalPod.DeepCopy()
			modifiedPod.Spec.NodeName = "worker-node-1"
			Expect(util.PodSpecChanged(modifiedPod, originalPod)).To(BeFalse())

			// Test 9: Default node lifecycle tolerations injected by Kubelet should be ignored
			modifiedPod = originalPod.DeepCopy()
			modifiedPod.Spec.Tolerations = append(modifiedPod.Spec.Tolerations,
				corev1.Toleration{
					Key:               "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				},
				corev1.Toleration{
					Key:               "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				},
			)
			Expect(util.PodSpecChanged(modifiedPod, originalPod)).To(BeFalse())
		})
	})
})
