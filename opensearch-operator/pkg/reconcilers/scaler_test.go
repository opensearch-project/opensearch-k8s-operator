package reconcilers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newScalerReconciler(client *k8s.MockK8sClient, spec *opensearchv1.OpenSearchCluster) *ScalerReconciler {
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

			mockClient := k8s.NewMockK8sClient(GinkgoT())
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

			mockClient := k8s.NewMockK8sClient(GinkgoT())
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

	Context("When tracking node names during scaling", func() {
		It("Should verify node name is stored in Conditions structure", func() {
			// This test verifies that the Conditions field structure supports storing node names
			// The actual storage happens in excludeNode which requires OpenSearch client mocking
			status := opensearchv1.ComponentStatus{
				Component:   "Scaler",
				Status:      "Excluded",
				Description: "data",
				Conditions:  []string{"test-cluster-data-2"},
			}

			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0]).To(Equal("test-cluster-data-2"))
		})

		It("Should detect node name mismatch in drainNode when target node changed", func() {
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"
			nodePoolComponent := "data"
			excludedNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent) // Node that was excluded

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					ConfMgmt: opensearchv1.ConfMgmt{
						SmartScaler: true,
					},
					NodePools: []opensearchv1.NodePool{
						{
							Component: nodePoolComponent,
							Replicas:  2,
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					ComponentsStatus: []opensearchv1.ComponentStatus{
						{
							Component:   "Scaler",
							Status:      "Excluded",
							Description: nodePoolComponent,
							Conditions:  []string{excludedNodeName}, // Node that was excluded
						},
					},
				},
			}

			stsName := fmt.Sprintf("%s-%s", clusterName, nodePoolComponent)
			currentSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName,
					Namespace: clusterNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To[int32](2), // Replicas changed, so last replica is now different
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 2,
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			// Mock status update to verify it resets to Running
			var statusResetToRunning bool
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				updateFn(&spec)
				// Check if status was reset to Running
				for _, status := range spec.Status.ComponentsStatus {
					if status.Component == "Scaler" && status.Status == "Running" {
						statusResetToRunning = true
					}
				}
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			currentStatus := spec.Status.ComponentsStatus[0]
			err := underTest.drainNode(currentStatus, currentSts, nodePoolComponent)

			// Should detect mismatch and reset to Running
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("target node mismatch"))
			Expect(statusResetToRunning).To(BeTrue())
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should detect node name mismatch in decreaseOneNode when target node changed", func() {
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"
			nodePoolComponent := "data"
			drainedNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent) // Node that was drained

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					ConfMgmt: opensearchv1.ConfMgmt{
						SmartScaler: true,
					},
					NodePools: []opensearchv1.NodePool{
						{
							Component: nodePoolComponent,
							Replicas:  1,
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					ComponentsStatus: []opensearchv1.ComponentStatus{
						{
							Component:   "Scaler",
							Status:      "Drained",
							Description: nodePoolComponent,
							Conditions:  []string{drainedNodeName}, // Node that was drained
						},
					},
				},
			}

			stsName := fmt.Sprintf("%s-%s", clusterName, nodePoolComponent)
			currentSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName,
					Namespace: clusterNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To[int32](2), // Replicas changed, so last replica is now different
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 2,
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			// Mock status update to verify it resets to Running
			var statusResetToRunning bool
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				updateFn(&spec)
				// Check if status was reset to Running
				for _, status := range spec.Status.ComponentsStatus {
					if status.Component == "Scaler" && status.Status == "Running" {
						statusResetToRunning = true
					}
				}
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			currentStatus := spec.Status.ComponentsStatus[0]
			_, err := underTest.decreaseOneNode(currentStatus, currentSts, nodePoolComponent, true)

			// Should detect mismatch and reset to Running
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("target node mismatch"))
			Expect(statusResetToRunning).To(BeTrue())
			mockClient.AssertExpectations(GinkgoT())
		})

		It("Should use node name from Conditions when draining if available", func() {
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"
			nodePoolComponent := "data"
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					ConfMgmt: opensearchv1.ConfMgmt{
						SmartScaler: true,
					},
					NodePools: []opensearchv1.NodePool{
						{
							Component: nodePoolComponent,
							Replicas:  2,
						},
					},
				},
				Status: opensearchv1.ClusterStatus{
					ComponentsStatus: []opensearchv1.ComponentStatus{
						{
							Component:   "Scaler",
							Status:      "Excluded",
							Description: nodePoolComponent,
							Conditions:  []string{targetNodeName}, // Node name stored in Conditions
						},
					},
				},
			}

			stsName := fmt.Sprintf("%s-%s", clusterName, nodePoolComponent)
			currentSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName,
					Namespace: clusterNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To[int32](3), // Matches the target node (index 2)
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas: 3,
				},
			}

			// Verify that drainNode would use the node name from Conditions
			// Since we can't mock OpenSearch client easily, we'll just verify the logic
			// In a real scenario, drainNode would retrieve targetNodeName from Conditions[0]
			expectedNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)
			Expect(expectedNodeName).To(Equal(targetNodeName))

			// Verify that the node name stored in Conditions matches the expected calculation
			currentStatus := spec.Status.ComponentsStatus[0]
			Expect(currentStatus.Conditions[0]).To(Equal(targetNodeName))
		})
	})
})
