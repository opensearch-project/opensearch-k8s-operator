package reconcilers

import (
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/stretchr/testify/mock"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

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

var _ = Describe("Node attributes RBAC reconciliation", func() {
	It("should not delete cluster role bindings when cluster-scoped RBAC management is disabled", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		instance := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rbac-test",
				Namespace: "test-namespace",
			},
		}
		underTest := &ClusterReconciler{
			client:   mockClient,
			instance: instance,
		}
		underTest.DisableClusterRoleBindingManagement()

		mockClient.EXPECT().
			ReconcileResource(mock.MatchedBy(func(obj runtime.Object) bool {
				sa, ok := obj.(*corev1.ServiceAccount)
				return ok &&
					sa.Name == builders.NodeAttributesServiceAccountName(instance) &&
					sa.Namespace == instance.Namespace
			}), reconciler.StateAbsent).
			Return(&ctrl.Result{}, nil)

		result := reconciler.CombinedResult{}
		shouldContinue := underTest.reconcileNodeAttributesRBAC(&result)

		Expect(shouldContinue).To(BeTrue())
		Expect(result.Err).NotTo(HaveOccurred())
	})

	It("should reject managed node attribute RBAC when cluster-scoped RBAC management is disabled", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		instance := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rbac-test",
				Namespace: "test-namespace",
			},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					NodeAttributes: []opensearchv1.NodeAttribute{
						{Name: "zone", NodeLabel: "topology.kubernetes.io/zone"},
					},
				},
			},
		}
		underTest := &ClusterReconciler{
			client:   mockClient,
			instance: instance,
		}
		underTest.DisableClusterRoleBindingManagement()

		result := reconciler.CombinedResult{}
		shouldContinue := underTest.reconcileNodeAttributesRBAC(&result)

		Expect(shouldContinue).To(BeFalse())
		Expect(result.Err).To(MatchError(ContainSubstring("requires ClusterRoleBinding management")))
	})

	It("should use the configured shared ClusterRole name", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		instance := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rbac-test",
				Namespace: "test-namespace",
			},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					NodeAttributes: []opensearchv1.NodeAttribute{
						{Name: "zone", NodeLabel: "topology.kubernetes.io/zone"},
					},
				},
			},
		}
		underTest := &ClusterReconciler{
			client:   mockClient,
			instance: instance,
		}
		underTest.SetNodeAttributesClusterRoleName("prefixed-node-attributes")

		mockClient.EXPECT().Scheme().Return(scheme.Scheme)
		mockClient.EXPECT().
			ReconcileResource(mock.MatchedBy(func(obj runtime.Object) bool {
				sa, ok := obj.(*corev1.ServiceAccount)
				return ok &&
					sa.Name == builders.NodeAttributesServiceAccountName(instance) &&
					sa.Namespace == instance.Namespace
			}), reconciler.StatePresent).
			Return(&ctrl.Result{}, nil)
		mockClient.On("EnsureClusterRoleBinding", mock.MatchedBy(func(crb *rbacv1.ClusterRoleBinding) bool {
			return crb.RoleRef.Name == "prefixed-node-attributes"
		})).Return(nil)

		result := reconciler.CombinedResult{}
		shouldContinue := underTest.reconcileNodeAttributesRBAC(&result)

		Expect(shouldContinue).To(BeTrue())
		Expect(result.Err).NotTo(HaveOccurred())
	})

	It("should not delete cluster role bindings during finalizer cleanup when cluster-scoped RBAC management is disabled", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		underTest := &ClusterReconciler{
			client:   mockClient,
			instance: &opensearchv1.OpenSearchCluster{},
		}
		underTest.DisableClusterRoleBindingManagement()

		result, err := underTest.DeleteResources()

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})
})
