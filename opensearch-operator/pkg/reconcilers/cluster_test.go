package reconcilers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/jarcoal/httpmock"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/patch"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	newPod := func(name string, uid types.UID, ready bool) corev1.Pod {
		status := corev1.PodStatus{}
		if ready {
			status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		}
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
			Status:     status,
		}
	}

	newPodAged := func(name string, uid types.UID, ready bool, created time.Time) corev1.Pod {
		pod := newPod(name, uid, ready)
		pod.CreationTimestamp = metav1.NewTime(created)
		return pod
	}

	classifyNow := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	It("counts a not-ready pod as existing when its UID matches the last recorded Ready UID (#1455)", func() {
		pods := []corev1.Pod{newPod("cluster-nodes-0", "uid-1", false)}
		recorded := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-1"},
		}

		existing, updates := classifyEmptyDirPods(pods, recorded, classifyNow)
		Expect(existing).To(Equal(1))
		Expect(updates).To(BeEmpty())
	})

	It("does not count a not-ready pod with a fresh UID as existing (#1526: force-delete recreation)", func() {
		// Simulates a StatefulSet-recreated pod: same name, new UID, not yet ready.
		pods := []corev1.Pod{newPod("cluster-nodes-0", "uid-2", false)}
		recorded := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-1"},
		}

		existing, updates := classifyEmptyDirPods(pods, recorded, classifyNow)
		Expect(existing).To(Equal(0))
		Expect(updates).To(BeEmpty())
	})

	It("records the UID of a Ready pod that has no prior record", func() {
		pods := []corev1.Pod{newPod("cluster-nodes-0", "uid-1", true)}

		existing, updates := classifyEmptyDirPods(pods, nil, classifyNow)
		Expect(existing).To(Equal(1))
		Expect(updates).To(Equal([]opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-1"},
		}))
	})

	It("counts a Ready pod with a new UID as existing and records the updated UID", func() {
		pods := []corev1.Pod{newPod("cluster-nodes-0", "uid-2", true)}
		recorded := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-1"},
		}

		existing, updates := classifyEmptyDirPods(pods, recorded, classifyNow)
		Expect(existing).To(Equal(1))
		Expect(updates).To(Equal([]opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-2"},
		}))
	})

	It("does not count a recently created not-ready pod with no prior UID record as existing", func() {
		pods := []corev1.Pod{newPodAged("cluster-nodes-0", "uid-1", false, classifyNow.Add(-time.Minute))}

		existing, updates := classifyEmptyDirPods(pods, nil, classifyNow)
		Expect(existing).To(Equal(0))
		Expect(updates).To(BeEmpty())
	})

	It("counts a long-lived not-ready pod with no prior UID record as existing (upgrade bootstrap)", func() {
		pods := []corev1.Pod{newPodAged("cluster-nodes-0", "uid-1", false, classifyNow.Add(-emptyDirRecoveryGracePeriod))}

		existing, updates := classifyEmptyDirPods(pods, nil, classifyNow)
		Expect(existing).To(Equal(1))
		Expect(updates).To(BeEmpty())
	})

	It("does not emit a UID update when a Ready pod already matches the recorded UID", func() {
		pods := []corev1.Pod{newPod("cluster-nodes-0", "uid-1", true)}
		recorded := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-1"},
		}

		existing, updates := classifyEmptyDirPods(pods, recorded, classifyNow)
		Expect(existing).To(Equal(1))
		Expect(updates).To(BeEmpty())
	})

	It("ignores terminating pods regardless of readiness or UID", func() {
		pod := newPod("cluster-nodes-0", "uid-1", true)
		now := metav1.Now()
		pod.DeletionTimestamp = &now

		existing, updates := classifyEmptyDirPods([]corev1.Pod{pod}, nil, classifyNow)
		Expect(existing).To(Equal(0))
		Expect(updates).To(BeEmpty())
	})

	It("reproduces #1526: all pods force-deleted and recreated trips data loss detection", func() {
		recorded := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "old-uid-0"},
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-1", Status: "old-uid-1"},
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-2", Status: "old-uid-2"},
		}
		// The StatefulSet has already recreated all three pods with fresh UIDs; none
		// have become Ready yet (they are still waiting to join the cluster).
		pods := []corev1.Pod{
			newPod("cluster-nodes-0", "new-uid-0", false),
			newPod("cluster-nodes-1", "new-uid-1", false),
			newPod("cluster-nodes-2", "new-uid-2", false),
		}

		existing, updates := classifyEmptyDirPods(pods, recorded, classifyNow)
		Expect(existing).To(Equal(0))
		Expect(updates).To(BeEmpty())

		stats := emptyDirPodStats{
			existingDataPods:   int32(existing),
			totalDataPods:      3,
			existingMasterPods: int32(existing),
			totalMasterPods:    3,
		}
		Expect(emptyDirDataLossSuspected(stats)).To(BeTrue())
	})

	It("does not wipe on operator upgrade while long-lived masters are NotReady (no UID records yet)", func() {
		// Simulates adopting this tracking while a majority of masters are crash-looping with intact emptyDirs (same pod objects for hours).
		pods := []corev1.Pod{
			newPodAged("cluster-nodes-0", "uid-0", true, classifyNow.Add(-time.Hour)),
			newPodAged("cluster-nodes-1", "uid-1", false, classifyNow.Add(-time.Hour)),
			newPodAged("cluster-nodes-2", "uid-2", false, classifyNow.Add(-time.Hour)),
		}

		existing, updates := classifyEmptyDirPods(pods, nil, classifyNow)
		Expect(existing).To(Equal(3))
		Expect(updates).To(Equal([]opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-0"},
		}))
		Expect(emptyDirDataLossSuspected(emptyDirPodStats{
			existingDataPods:   int32(existing),
			totalDataPods:      3,
			existingMasterPods: int32(existing),
			totalMasterPods:    3,
		})).To(BeFalse())
	})

	It("upsertComponentStatus replaces an existing entry in place and appends otherwise", func() {
		components := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "old-uid"},
		}

		components = upsertComponentStatus(components, opensearchv1.ComponentStatus{
			Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "new-uid",
		})
		Expect(components).To(HaveLen(1))
		Expect(components[0].Status).To(Equal("new-uid"))

		components = upsertComponentStatus(components, opensearchv1.ComponentStatus{
			Component: emptyDirPodUIDComponent, Description: "cluster-nodes-1", Status: "uid-1",
		})
		Expect(components).To(HaveLen(2))
	})

	It("removeComponentStatusesByComponent clears EmptyDirRecovery regardless of timestamp", func() {
		components := []opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-0"},
			{Component: emptyDirRecoveryComponent, Status: emptyDirRecoveryStatusPending, Description: classifyNow.Format(time.RFC3339)},
		}

		components = removeComponentStatusesByComponent(components, emptyDirRecoveryComponent)
		Expect(components).To(Equal([]opensearchv1.ComponentStatus{
			{Component: emptyDirPodUIDComponent, Description: "cluster-nodes-0", Status: "uid-0"},
		}))
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

		It("should ignore admission controller drift when last-applied spec is unchanged", func() {
			instance := bootstrapTestCluster("admission-drift-test")
			desired := builders.NewBootstrapPod(instance, nil, nil)

			existing := desired.DeepCopy()
			Expect(patch.DefaultAnnotator.SetLastAppliedAnnotation(existing)).To(Succeed())
			simulateAdmissionControllerDrift(existing)

			Expect(util.BootstrapPodNeedsRecreation(existing, desired)).To(BeFalse())
		})

		It("should recreate when the operator desired spec has changed", func() {
			instance := bootstrapTestCluster("spec-change-test")
			original := builders.NewBootstrapPod(instance, nil, nil)

			existing := original.DeepCopy()
			Expect(patch.DefaultAnnotator.SetLastAppliedAnnotation(existing)).To(Succeed())

			desired := original.DeepCopy()
			desired.Spec.ServiceAccountName = "updated-sa"
			Expect(util.BootstrapPodNeedsRecreation(existing, desired)).To(BeTrue())
		})

		It("should not recreate when last-applied annotation is missing", func() {
			instance := bootstrapTestCluster("missing-annotation-test")
			desired := builders.NewBootstrapPod(instance, nil, nil)
			existing := desired.DeepCopy()
			simulateAdmissionControllerDrift(existing)

			Expect(util.BootstrapPodNeedsRecreation(existing, desired)).To(BeFalse())
		})
	})

	Context("reconcileBootstrapPod", func() {
		It("should create the pod when it does not exist", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			instance := bootstrapTestCluster("create-test")
			desired := builders.NewBootstrapPod(instance, nil, nil)
			underTest := &ClusterReconciler{client: mockClient, instance: instance}

			mockClient.EXPECT().
				GetPod(desired.Name, desired.Namespace).
				Return(corev1.Pod{}, k8serrors.NewNotFound(schema.GroupResource{Resource: "pods"}, desired.Name))
			mockClient.EXPECT().
				ReconcileResource(desired, reconciler.StateCreated).
				Return(&ctrl.Result{}, nil)

			result, err := underTest.reconcileBootstrapPod(desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{}))
		})

		It("should not patch or recreate when admission controllers mutated the live spec", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			instance := bootstrapTestCluster("drift-reconcile-test")
			desired := builders.NewBootstrapPod(instance, nil, nil)
			existing := desired.DeepCopy()
			Expect(patch.DefaultAnnotator.SetLastAppliedAnnotation(existing)).To(Succeed())
			simulateAdmissionControllerDrift(existing)

			underTest := &ClusterReconciler{client: mockClient, instance: instance}
			mockClient.EXPECT().
				GetPod(desired.Name, desired.Namespace).
				Return(*existing, nil)

			result, err := underTest.reconcileBootstrapPod(desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{}))
		})

		It("should recreate the pod when the operator desired spec has changed", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			instance := bootstrapTestCluster("recreate-test")
			original := builders.NewBootstrapPod(instance, nil, nil)
			existing := original.DeepCopy()
			Expect(patch.DefaultAnnotator.SetLastAppliedAnnotation(existing)).To(Succeed())

			desired := original.DeepCopy()
			desired.Spec.ServiceAccountName = "updated-sa"

			underTest := &ClusterReconciler{client: mockClient, instance: instance}
			mockClient.EXPECT().
				GetPod(desired.Name, desired.Namespace).
				Return(*existing, nil)
			mockClient.EXPECT().DeletePod(mock.Anything).Return(nil)
			mockClient.EXPECT().WaitForPodDeletion(desired.Name, desired.Namespace).Return(nil)
			mockClient.EXPECT().
				ReconcileResource(desired, reconciler.StateCreated).
				Return(&ctrl.Result{}, nil)

			result, err := underTest.reconcileBootstrapPod(desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{}))
		})

		It("should requeue when the existing bootstrap pod is terminating", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())
			instance := bootstrapTestCluster("terminating-test")
			desired := builders.NewBootstrapPod(instance, nil, nil)
			existing := desired.DeepCopy()
			now := metav1.Now()
			existing.DeletionTimestamp = &now

			underTest := &ClusterReconciler{client: mockClient, instance: instance}
			mockClient.EXPECT().
				GetPod(desired.Name, desired.Namespace).
				Return(*existing, nil)

			result, err := underTest.reconcileBootstrapPod(desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Second}))
		})
	})
})

func bootstrapTestCluster(name string) *opensearchv1.OpenSearchCluster {
	return &opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test-namespace",
		},
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{
				HttpPort:       9200,
				ServiceName:    name,
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
}

func simulateAdmissionControllerDrift(pod *corev1.Pod) {
	pod.Spec.SchedulerName = "gke-custom-scheduler"
	injected := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}
	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i].Resources = injected
	}
}

var _ = Describe("ServiceMonitor reconciliation", func() {
	newMonitoringInstance := func() *opensearchv1.OpenSearchCluster {
		return &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitoring-test",
				Namespace: "test-namespace",
			},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					Monitoring: opensearchv1.MonitoringConfig{Enable: true},
				},
			},
		}
	}

	It("should not fail the reconcile when monitoring is enabled but the ServiceMonitor CRD is missing", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		instance := newMonitoringInstance()
		underTest := &ClusterReconciler{
			client:   mockClient,
			instance: instance,
			recorder: record.NewFakeRecorder(1),
		}

		mockClient.EXPECT().Scheme().Return(scheme.Scheme)
		mockClient.EXPECT().
			ReconcileResource(mock.AnythingOfType("*v1.ServiceMonitor"), reconciler.StatePresent).
			Return(nil, &apimeta.NoKindMatchError{
				GroupKind: schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"},
			})

		result := reconciler.CombinedResult{}
		underTest.reconcileServiceMonitor(&result)

		Expect(result.Err).NotTo(HaveOccurred())
	})

	It("should surface other ServiceMonitor errors", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		instance := newMonitoringInstance()
		underTest := &ClusterReconciler{
			client:   mockClient,
			instance: instance,
			recorder: record.NewFakeRecorder(1),
		}

		mockClient.EXPECT().Scheme().Return(scheme.Scheme)
		mockClient.EXPECT().
			ReconcileResource(mock.AnythingOfType("*v1.ServiceMonitor"), reconciler.StatePresent).
			Return(nil, errors.New("some other failure"))

		result := reconciler.CombinedResult{}
		underTest.reconcileServiceMonitor(&result)

		Expect(result.Err).To(MatchError(ContainSubstring("some other failure")))
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

var _ = Describe("StatefulSet recreation on immutable field change", func() {
	newInstance := func() *opensearchv1.OpenSearchCluster {
		return &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "os", Namespace: "test-namespace"},
		}
	}
	newSTS := func(selector map[string]string, policy appsv1.PodManagementPolicyType) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "os-nodes", Namespace: "test-namespace"},
			Spec: appsv1.StatefulSetSpec{
				Replicas:            ptr.To(int32(3)),
				Selector:            &metav1.LabelSelector{MatchLabels: selector},
				PodManagementPolicy: policy,
			},
		}
	}
	legacySelector := map[string]string{
		helpers.OldClusterLabel:  "os",
		helpers.OldNodePoolLabel: "nodes",
	}
	newSelector := map[string]string{
		helpers.ClusterLabel:  "os",
		helpers.NodePoolLabel: "nodes",
	}

	It("explains the one-time rolling restart when adopting a 2.x StatefulSet (#1580)", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recorder := record.NewFakeRecorder(1)
		underTest := &ClusterReconciler{client: mockClient, instance: newInstance(), recorder: recorder}

		existing := newSTS(legacySelector, appsv1.OrderedReadyPodManagement)
		existing.Spec.Template.Labels = legacySelector
		desired := newSTS(newSelector, appsv1.ParallelPodManagement)
		desired.Spec.Template.Labels = newSelector

		mockClient.EXPECT().DeleteStatefulSet(existing, true).Return(nil)
		mockClient.EXPECT().ReconcileResource(desired, reconciler.StatePresent).Return(nil, nil)

		_, err := underTest.recreateSTSForImmutableFieldChange(existing, desired)
		Expect(err).NotTo(HaveOccurred())

		Expect(recorder.Events).To(HaveLen(1))
		event := <-recorder.Events
		Expect(event).To(HavePrefix("Warning StatefulSetRecreated"))
		Expect(event).To(ContainSubstring("test-namespace/os-nodes"))
		Expect(event).To(ContainSubstring("selector changed"))
		Expect(event).To(ContainSubstring("API group migration"))
		Expect(event).To(ContainSubstring("rolling-restarted once"))
		Expect(event).NotTo(ContainSubstring("under the opensearch.org API group"))
	})

	It("emits a generic reason and does not promise a restart when the pod template is unchanged", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recorder := record.NewFakeRecorder(1)
		underTest := &ClusterReconciler{client: mockClient, instance: newInstance(), recorder: recorder}

		existing := newSTS(newSelector, appsv1.OrderedReadyPodManagement)
		desired := newSTS(newSelector, appsv1.ParallelPodManagement)

		mockClient.EXPECT().DeleteStatefulSet(existing, true).Return(nil)
		mockClient.EXPECT().ReconcileResource(desired, reconciler.StatePresent).Return(nil, nil)

		_, err := underTest.recreateSTSForImmutableFieldChange(existing, desired)
		Expect(err).NotTo(HaveOccurred())

		event := <-recorder.Events
		Expect(event).To(ContainSubstring("an immutable field changed"))
		Expect(event).NotTo(ContainSubstring("selector changed"))
		Expect(event).NotTo(ContainSubstring("rolling-restarted"))
		Expect(event).NotTo(ContainSubstring("version upgrade"))
	})

	It("promises a rolling restart when the pod template changed and the selector did not", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recorder := record.NewFakeRecorder(1)
		underTest := &ClusterReconciler{client: mockClient, instance: newInstance(), recorder: recorder}

		existing := newSTS(newSelector, appsv1.ParallelPodManagement)
		desired := newSTS(newSelector, appsv1.ParallelPodManagement)
		desired.Spec.Template.Annotations = map[string]string{"restarted": "true"}

		mockClient.EXPECT().DeleteStatefulSet(existing, true).Return(nil)
		mockClient.EXPECT().ReconcileResource(desired, reconciler.StatePresent).Return(nil, nil)

		_, err := underTest.recreateSTSForImmutableFieldChange(existing, desired)
		Expect(err).NotTo(HaveOccurred())

		event := <-recorder.Events
		Expect(event).To(ContainSubstring("an immutable field changed"))
		Expect(event).To(ContainSubstring("rolling-restarted once"))
	})

	It("attributes the restart to the version upgrade when one is in progress", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recorder := record.NewFakeRecorder(1)
		instance := newInstance()
		instance.Status.Version = "2.19.0"
		instance.Spec.General.Version = "3.0.0"
		underTest := &ClusterReconciler{client: mockClient, instance: instance, recorder: recorder}

		existing := newSTS(legacySelector, appsv1.OrderedReadyPodManagement)
		existing.Spec.Template.Labels = legacySelector
		desired := newSTS(newSelector, appsv1.ParallelPodManagement)
		desired.Spec.Template.Labels = newSelector

		mockClient.EXPECT().DeleteStatefulSet(existing, true).Return(nil)
		mockClient.EXPECT().ReconcileResource(desired, reconciler.StatePresent).Return(nil, nil)

		_, err := underTest.recreateSTSForImmutableFieldChange(existing, desired)
		Expect(err).NotTo(HaveOccurred())

		event := <-recorder.Events
		Expect(event).To(ContainSubstring("in-progress OpenSearch version upgrade"))
		Expect(event).NotTo(ContainSubstring("rolling-restarted"))
	})

	It("does not recreate the StatefulSet when the orphaning delete fails", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recorder := record.NewFakeRecorder(1)
		underTest := &ClusterReconciler{client: mockClient, instance: newInstance(), recorder: recorder}

		existing := newSTS(legacySelector, appsv1.OrderedReadyPodManagement)
		desired := newSTS(newSelector, appsv1.ParallelPodManagement)

		mockClient.EXPECT().DeleteStatefulSet(existing, true).Return(errors.New("conflict"))

		_, err := underTest.recreateSTSForImmutableFieldChange(existing, desired)
		Expect(err).To(MatchError("conflict"))
		Expect(recorder.Events).To(HaveLen(0))
	})

	It("does not emit StatefulSetRecreated when creating the replacement fails", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recorder := record.NewFakeRecorder(1)
		underTest := &ClusterReconciler{client: mockClient, instance: newInstance(), recorder: recorder}

		existing := newSTS(legacySelector, appsv1.OrderedReadyPodManagement)
		desired := newSTS(newSelector, appsv1.ParallelPodManagement)

		mockClient.EXPECT().DeleteStatefulSet(existing, true).Return(nil)
		mockClient.EXPECT().ReconcileResource(desired, reconciler.StatePresent).Return(nil, errors.New("create failed"))

		_, err := underTest.recreateSTSForImmutableFieldChange(existing, desired)
		Expect(err).To(MatchError("create failed"))
		Expect(recorder.Events).To(HaveLen(0))
	})

	It("selects an adopted pod whose controller-revision-hash differs from the recreated StatefulSet", func() {
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		recreated := newSTS(newSelector, appsv1.ParallelPodManagement)
		recreated.Status.UpdateRevision = "os-nodes-new-rev"

		mockClient.EXPECT().GetPod("os-nodes-0", "test-namespace").Return(corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "os-nodes-0",
				Namespace: "test-namespace",
				Labels: map[string]string{
					helpers.ClusterLabel:       "os",
					helpers.NodePoolLabel:      "nodes",
					"controller-revision-hash": "os-nodes-old-rev",
				},
			},
		}, nil)

		pod, err := helpers.GetPodWithOlderRevision(mockClient, recreated)
		Expect(err).NotTo(HaveOccurred())
		Expect(pod).NotTo(BeNil())
		Expect(pod.Name).To(Equal("os-nodes-0"))
	})
})

var _ = Describe("Bootstrap pod voting-config exclusion (issue #1448)", func() {
	const (
		clusterName      = "test-cluster"
		clusterNamespace = "test-namespace"
	)

	newCluster := func() *opensearchv1.OpenSearchCluster {
		return &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace, UID: "dummyuid"},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{ServiceName: clusterName, HttpPort: 9200},
			},
			Status: opensearchv1.ClusterStatus{Initialized: true},
		}
	}

	It("Should POST a voting exclusion, delete the pod, then waiting-DELETE exclusions once the node has left", func() {
		instance := newCluster()
		bootstrapPod := builders.NewBootstrapPod(instance, nil, nil)
		transport := httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		registerOsPingResponders(transport, instance)
		registerCatNodesSequence(transport, []string{builders.BootstrapPodName(instance)}, []string{})
		registerVotingExclusionsState(transport, builders.BootstrapPodName(instance))
		calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

		mockClient := k8s.NewMockK8sClient(GinkgoT())
		mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
		mockClient.On("GetPod", bootstrapPod.Name, bootstrapPod.Namespace).Return(*bootstrapPod, nil)
		mockClient.On("ReconcileResource", mock.Anything, reconciler.StateAbsent).Return(&ctrl.Result{}, nil)

		underTest := &ClusterReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			instance:          instance,
			logger:            logr.Discard(),
			osClientTransport: transport,
		}
		result, err := underTest.removeBootstrapPod(bootstrapPod)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(&ctrl.Result{}))
		Expect(*calls).To(Equal([]string{
			"POST node_names=" + builders.BootstrapPodName(instance) + "&timeout=10s",
			"DELETE wait_for_removal=true",
		}))
	})

	It("Should POST, delete the pod and leave the clear to the scaler sweep while the bootstrap node is still a member", func() {
		instance := newCluster()
		bootstrapPod := builders.NewBootstrapPod(instance, nil, nil)
		transport := httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		registerOsPingResponders(transport, instance)
		registerCatNodesResponder(transport, builders.BootstrapPodName(instance))
		calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

		mockClient := k8s.NewMockK8sClient(GinkgoT())
		mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
		mockClient.On("GetPod", bootstrapPod.Name, bootstrapPod.Namespace).Return(*bootstrapPod, nil)
		mockClient.On("ReconcileResource", mock.Anything, reconciler.StateAbsent).Return(&ctrl.Result{}, nil)

		underTest := &ClusterReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			instance:          instance,
			logger:            logr.Discard(),
			osClientTransport: transport,
		}
		result, err := underTest.removeBootstrapPod(bootstrapPod)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(&ctrl.Result{}))
		Expect(*calls).To(Equal([]string{"POST node_names=" + builders.BootstrapPodName(instance) + "&timeout=10s"}))
	})

	It("Should leave the bootstrap pod and not fail the reconciler if the voting-config POST fails", func() {
		instance := newCluster()
		bootstrapPod := builders.NewBootstrapPod(instance, nil, nil)
		transport := httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		registerOsPingResponders(transport, instance)
		registerCatNodesResponder(transport, builders.BootstrapPodName(instance))
		calls := recordVotingConfigCalls(transport, http.StatusInternalServerError, http.StatusOK)

		mockClient := k8s.NewMockK8sClient(GinkgoT())
		mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
		mockClient.On("GetPod", bootstrapPod.Name, bootstrapPod.Namespace).Return(*bootstrapPod, nil)

		underTest := &ClusterReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			instance:          instance,
			logger:            logr.Discard(),
			osClientTransport: transport,
		}
		result, err := underTest.removeBootstrapPod(bootstrapPod)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))
		Expect(result.Requeue).To(BeFalse())
		Expect(*calls).To(HaveLen(1))
		Expect((*calls)[0]).To(HavePrefix("POST "))
		Expect(*calls).NotTo(ContainElement(ContainSubstring("wait_for_removal=false")))
		mockClient.AssertNotCalled(GinkgoT(), "ReconcileResource", mock.Anything, mock.Anything)
	})
})
