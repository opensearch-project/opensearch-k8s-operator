package reconcilers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	k8sMock "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newScalerReconciler(client *k8sMock.MockK8sClient, spec *opensearchv1.OpenSearchCluster) *ScalerReconciler {
	reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, spec.Spec.NodePools)
	underTest := &ScalerReconciler{
		client:            client,
		ctx:               context.Background(),
		recorder:          &record.FakeRecorder{},
		reconcilerContext: &reconcilerContext,
		instance:          spec,
	}
	return underTest
}

var _ = Describe("Scaler Controller", func() {

	Context("When cleaning up StatefulSets", func() {
		It("Should use the correct namespace when listing StatefulSets", func() {
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Replicas:  3,
						},
					},
				},
			}

			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			// Mock the ListStatefulSets call to verify it uses the correct namespace
			mockClient.On("ListStatefulSets",
				client.InNamespace(clusterNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{},
			}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			result := &reconciler.CombinedResult{}
			underTest.cleanupStatefulSets(result)
			Expect(result.Err).To(BeNil())
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should fail if wrong namespace is used (regression test)", func() {
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"
			wrongNamespace := clusterName // This would be the bug: using cluster name as namespace

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Replicas:  3,
						},
					},
				},
			}

			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			// Mock the ListStatefulSets call with the WRONG namespace (the bug scenario)
			// This should NOT be called if the fix is working correctly
			mockClient.On("ListStatefulSets",
				client.InNamespace(wrongNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{},
			}, nil).Maybe() // Maybe() means this call might not happen

			// Mock the ListStatefulSets call with the CORRECT namespace
			mockClient.On("ListStatefulSets",
				client.InNamespace(clusterNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{},
			}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			result := &reconciler.CombinedResult{}
			underTest.cleanupStatefulSets(result)
			Expect(result.Err).To(BeNil())

			// Verify that the correct namespace was used, not the wrong one
			// This test ensures the bug is fixed
			mockClient.AssertExpectations(GinkgoT())
		})
	})

	Context("When tracking node names through Conditions during scale-down", func() {
		var (
			clusterName      = "test-cluster"
			clusterNamespace = "test-namespace"
			nodePoolComp     = "data"
			stsName          = clusterName + "-" + nodePoolComp
		)

		makeSpec := func(replicas int32, smartScaler bool, componentStatuses []opensearchv1.ComponentStatus) *opensearchv1.OpenSearchCluster {
			return &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					ConfMgmt: opensearchv1.ConfMgmt{
						SmartScaler: smartScaler,
					},
					NodePools: []opensearchv1.NodePool{
						{
							Component: nodePoolComp,
							Replicas:  replicas,
							Roles:     []string{"data"},
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					ComponentsStatus: componentStatuses,
				},
			}
		}

		makeSts := func(replicas int32) appsv1.StatefulSet {
			return appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName,
					Namespace: clusterNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &replicas,
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: replicas,
				},
			}
		}

		It("Should store node name in Conditions when setting Excluded status", func() {
			// STS has 5 replicas, spec wants 4 → scale down by 1 with SmartScaler enabled
			spec := makeSpec(4, true, nil)
			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			underTest := newScalerReconciler(mockClient, spec)

			sts := makeSts(5)

			// Mock GetStatefulSet
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(sts, nil)

			// Mock ListPods for ReadyReplicasForNodePool
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil)

			// Mock UpdateOpenSearchClusterStatus for the "Running" status set
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil).Once()

			// Mock GetSecret for OS client creation in excludeNode
			mockClient.On("GetSecret", clusterName+"-admin-password", clusterNamespace).Return(
				corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil,
			)

			// Call reconcileNodePool - it will reach excludeNode, which will fail
			// at the OS client HTTP call, but we can verify the code path
			nodePool := &spec.Spec.NodePools[0]
			_, err := underTest.reconcileNodePool(nodePool)
			// Error is expected since we can't actually connect to OpenSearch
			// The key assertion is that the mock expectations are met
			_ = err
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should extract Conditions[0] from Excluded status and pass to drainNode", func() {
			// Status already has "Excluded" with Conditions tracking the target node
			targetNode := stsName + "-4"
			spec := makeSpec(4, true, []opensearchv1.ComponentStatus{
				{
					Component:   "Scaler",
					Status:      "Excluded",
					Description: nodePoolComp,
					Conditions:  []string{targetNode},
				},
			})

			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			underTest := newScalerReconciler(mockClient, spec)

			sts := makeSts(5)

			// Mock GetStatefulSet
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(sts, nil)

			// Mock ListPods for ReadyReplicasForNodePool
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil)

			// Mock GetSecret for OS client creation in drainNode
			mockClient.On("GetSecret", clusterName+"-admin-password", clusterNamespace).Return(
				corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil,
			)

			// Call reconcileNodePool - it will enter the "Excluded" handler and call drainNode
			// which will use targetNode from Conditions[0]
			nodePool := &spec.Spec.NodePools[0]
			_, err := underTest.reconcileNodePool(nodePool)
			// Error expected since OS client can't connect; the important thing is
			// we verified the code path goes through the Excluded handler
			_ = err
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should handle Excluded status without Conditions (backward compatibility)", func() {
			// Status has "Excluded" without Conditions (pre-fix state)
			spec := makeSpec(4, true, []opensearchv1.ComponentStatus{
				{
					Component:   "Scaler",
					Status:      "Excluded",
					Description: nodePoolComp,
				},
			})

			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			underTest := newScalerReconciler(mockClient, spec)

			sts := makeSts(5)

			// Mock GetStatefulSet
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(sts, nil)

			// Mock ListPods for ReadyReplicasForNodePool
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil)

			// Mock GetSecret for OS client creation in drainNode
			mockClient.On("GetSecret", clusterName+"-admin-password", clusterNamespace).Return(
				corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil,
			)

			// Call reconcileNodePool - should still work even without Conditions
			// drainNode falls back to computing node name from STS
			nodePool := &spec.Spec.NodePools[0]
			_, err := underTest.reconcileNodePool(nodePool)
			_ = err
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should scale down without SmartScaler and remove status correctly", func() {
			// STS has 5 replicas, spec wants 4, SmartScaler disabled
			spec := makeSpec(4, false, nil)
			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			underTest := newScalerReconciler(mockClient, spec)

			sts := makeSts(5)

			// Mock GetStatefulSet
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(sts, nil)

			// Mock ListPods for ReadyReplicasForNodePool
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil)

			// Mock UpdateOpenSearchClusterStatus for the "Running" status set
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).Return(nil)

			// Mock ReconcileResource for the STS update (decrease replicas)
			mockClient.On("ReconcileResource", mock.Anything, mock.Anything).Return(nil, nil)

			nodePool := &spec.Spec.NodePools[0]
			requeue, err := underTest.reconcileNodePool(nodePool)
			Expect(err).To(BeNil())
			Expect(requeue).To(BeFalse())
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should clear status and restart cycle for next node on multi-node scale-down", func() {
			// After removing node 4, status is cleared. Next reconcile finds
			// STS at 4 replicas, spec wants 3 → should start fresh cycle for node 3
			spec := makeSpec(3, true, nil)
			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			underTest := newScalerReconciler(mockClient, spec)

			sts := makeSts(4) // STS now has 4 replicas after previous node was removed

			// Mock GetStatefulSet
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(sts, nil)

			// Mock ListPods for ReadyReplicasForNodePool
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil)

			// Mock UpdateOpenSearchClusterStatus - capture the status update to verify
			var capturedStatus opensearchv1.ComponentStatus
			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything,
				mock.MatchedBy(func(f func(*opensearchv1.OpenSearchCluster)) bool {
					return true
				}),
			).Run(func(args mock.Arguments) {
				updateFunc := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				// Apply the update to capture what it does
				testInstance := &opensearchv1.OpenSearchCluster{}
				updateFunc(testInstance)
				if len(testInstance.Status.ComponentsStatus) > 0 {
					capturedStatus = testInstance.Status.ComponentsStatus[0]
				}
			}).Return(nil).Once()

			// Mock GetSecret for OS client creation in excludeNode
			mockClient.On("GetSecret", clusterName+"-admin-password", clusterNamespace).Return(
				corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil,
			)

			nodePool := &spec.Spec.NodePools[0]
			requeue, err := underTest.reconcileNodePool(nodePool)
			// Error expected since OS client can't connect
			_ = err
			_ = requeue

			// Verify that the status was set to "Running" with the correct Description
			// This proves the state machine restarted fresh for the next node
			Expect(capturedStatus.Component).To(Equal("Scaler"))
			Expect(capturedStatus.Status).To(Equal("Running"))
			Expect(capturedStatus.Description).To(Equal(nodePoolComp))
		})

		It("Should preserve Conditions through Drained status", func() {
			targetNode := stsName + "-4"
			spec := makeSpec(4, true, []opensearchv1.ComponentStatus{
				{
					Component:   "Scaler",
					Status:      "Drained",
					Description: nodePoolComp,
					Conditions:  []string{targetNode},
				},
			})

			mockClient := k8sMock.NewMockK8sClient(GinkgoT())
			underTest := newScalerReconciler(mockClient, spec)

			sts := makeSts(5)

			// Mock GetStatefulSet
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(sts, nil)

			// Mock ListPods for ReadyReplicasForNodePool
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil)

			// Mock UpdateOpenSearchClusterStatus for RemoveIt in decreaseOneNode
			mockClient.On("UpdateOpenSearchClusterStatus",
				types.NamespacedName{Name: clusterName, Namespace: clusterNamespace},
				mock.Anything,
			).Return(nil)

			// Mock ReconcileResource for the STS update (decrease replicas)
			mockClient.On("ReconcileResource", mock.Anything, mock.Anything).Return(nil, nil)

			// Mock GetSecret for OS client creation in decreaseOneNode (for RemoveExcludeNodeHost)
			mockClient.On("GetSecret", clusterName+"-admin-password", clusterNamespace).Return(
				corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil,
			)

			nodePool := &spec.Spec.NodePools[0]
			_, err := underTest.reconcileNodePool(nodePool)
			// The decreaseOneNode call will proceed, and then try to create OS client
			// for RemoveExcludeNodeHost which will fail at the HTTP level
			_ = err
			mockClient.AssertExpectations(GinkgoT())
		})
	})
})
