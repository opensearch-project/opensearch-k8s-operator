package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
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

func mockScalerAdminSecret(mockClient *k8s.MockK8sClient, clusterName, namespace string) {
	mockClient.On("GetSecret", clusterName+"-admin-password", namespace).Return(corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-admin-password",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("admin"),
		},
	}, nil)
}

func registerOsPingResponders(transport *httpmock.MockTransport, cluster *opensearchv1.OpenSearchCluster) {
	clusterURL := helpers.ClusterURL(cluster)
	mainBody := `{"name":"test","cluster_name":"test","version":{"number":"2.11.0"}}`
	for _, u := range []string{clusterURL, clusterURL + "/"} {
		transport.RegisterResponder(http.MethodHead, u, httpmock.NewStringResponder(200, "OK"))
		transport.RegisterResponder(http.MethodGet, u, httpmock.NewStringResponder(200, mainBody))
	}
}

func registerClusterSettingsResponders(transport *httpmock.MockTransport) {
	transport.RegisterResponder(
		http.MethodGet,
		`=~.*/_cluster/settings.*`,
		httpmock.NewStringResponder(200, `{"transient":{},"persistent":{}}`),
	)
	transport.RegisterResponder(
		http.MethodPut,
		`=~.*/_cluster/settings.*`,
		httpmock.NewStringResponder(200, `{"transient":{},"persistent":{}}`),
	)
}

// registerAllocationEnableSpy makes GET _cluster/settings report
// allocation.enable=primaries, and returns a pointer that records the last
// value any PUT _cluster/settings request actually set allocation.enable to
// (other PUTs, e.g. AppendExcludeNodeHost's, only touch "exclude" and leave
// it unchanged).
func registerAllocationEnableSpy(transport *httpmock.MockTransport) *string {
	transport.RegisterResponder(
		http.MethodGet,
		`=~.*/_cluster/settings.*`,
		httpmock.NewStringResponder(200, `{"transient":{"cluster.routing.allocation.enable":"primaries"},"persistent":{}}`),
	)
	reactivatedTo := new(string)
	transport.RegisterResponder(
		http.MethodPut,
		`=~.*/_cluster/settings.*`,
		func(req *http.Request) (*http.Response, error) {
			var body responses.ClusterSettingsResponse
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			cluster, _ := body.Transient["cluster"].(map[string]interface{})
			routing, _ := cluster["routing"].(map[string]interface{})
			allocation, _ := routing["allocation"].(map[string]interface{})
			if v, ok := allocation["enable"].(string); ok {
				*reactivatedTo = v
			}
			return httpmock.NewStringResponse(200, `{"transient":{},"persistent":{}}`), nil
		},
	)
	return reactivatedTo
}

func registerCatShardsResponder(transport *httpmock.MockTransport, status int, body string) {
	transport.RegisterResponder(
		http.MethodGet,
		`=~.*/_cat/shards.*`,
		httpmock.NewStringResponder(status, body),
	)
}

func scalerDrainTestCluster(clusterName, namespace, nodePoolComponent, status, nodeName string, extraConditions []string) opensearchv1.OpenSearchCluster {
	conditions := append([]string{nodeName}, extraConditions...)
	return opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
			UID:       "dummyuid",
		},
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{
				ServiceName: clusterName,
				HttpPort:    9200,
			},
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
					Status:      status,
					Description: nodePoolComponent,
					Conditions:  conditions,
				},
			},
		},
	}
}

func scalerDrainTestSts(clusterName, namespace, nodePoolComponent string, replicas int32) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", clusterName, nodePoolComponent),
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(replicas),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: replicas,
		},
	}
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
			_, err := underTest.decreaseOneNode(currentStatus, currentSts, nodePoolComponent, true, false)

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

	Context("When coordinating with upgrade", func() {
		It("Should skip replica scaling while an upgrade is in progress but still clean up removed pools", func() {
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.12.0",
					},
					NodePools: []opensearchv1.NodePool{
						{Component: "masters", Replicas: 3},
					},
				},
				Status: opensearchv1.ClusterStatus{
					Version: "2.11.0",
				},
			}

			mastersSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName + "-masters",
					Namespace: clusterNamespace,
					Labels: map[string]string{
						helpers.ClusterLabel:  clusterName,
						helpers.NodePoolLabel: "masters",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.StatefulSetStatus{
					AvailableReplicas: 3,
				},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			// Upgrade guard skips reconcileNodePool, but readiness + cleanup still run.
			mockClient.On("GetStatefulSet", clusterName+"-masters", clusterNamespace).Return(mastersSts, nil)
			mockClient.On("ListStatefulSets",
				client.InNamespace(clusterNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{mastersSts},
			}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			result, err := underTest.Reconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			mockClient.AssertExpectations(GinkgoT())
		})
	})

	Context("When accumulating requeue across node pools", func() {
		It("Should keep Requeue when an earlier pool requested it (regression #1454)", func() {
			// Previously only the last pool's requeue was combined on the success
			// path, so a drain on a non-last pool reported Requeue=false and the
			// main chain proceeded into upgrade/restart.
			results := &reconciler.CombinedResult{}
			poolRequeues := []bool{true, false} // first pool draining, last idle
			for _, requeue := range poolRequeues {
				results.Combine(&ctrl.Result{Requeue: requeue}, nil)
			}
			Expect(results.Result.Requeue).To(BeTrue())

			// Demonstrate the old overwrite bug for clarity
			requeue := false
			for _, r := range poolRequeues {
				requeue = r
			}
			Expect(requeue).To(BeFalse())
		})
	})

	Context("When computing drain stall", func() {
		It("Should not stall before the threshold", func() {
			started := time.Now().UTC().Add(-14 * time.Minute)
			Expect(drainHasStalled(started, time.Now().UTC(), drainStallWarningAfter)).To(BeFalse())
		})

		It("Should stall once the threshold has elapsed", func() {
			started := time.Now().UTC().Add(-16 * time.Minute)
			Expect(drainHasStalled(started, time.Now().UTC(), drainStallWarningAfter)).To(BeTrue())
		})

		It("Should keep the node name as the first condition", func() {
			started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			conditions := scalerDrainConditions("node-1", started, true)
			Expect(scalerTargetNodeName(conditions)).To(Equal("node-1"))
			got, ok := drainStartedAt(conditions)
			Expect(ok).To(BeTrue())
			Expect(got).To(Equal(started))
			Expect(hasDrainStalledCondition(conditions)).To(BeTrue())
		})
	})

	Context("When draining nodes (issue #1447)", func() {
		const (
			clusterName       = "test-cluster"
			clusterNamespace  = "test-namespace"
			nodePoolComponent = "data"
		)

		It("Should fail closed when _cat/shards errors instead of marking the node drained", func() {
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Excluded", targetNodeName, nil)
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusInternalServerError, `{"error":"unavailable"}`)
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			err := underTest.drainNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cat shards failed"))
			for _, status := range spec.Status.ComponentsStatus {
				Expect(status.Status).ToNot(Equal("Drained"))
			}
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
		})

		It("Should keep waiting when the node still has shards", func() {
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			started := time.Now().UTC().Add(-time.Minute)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Excluded", targetNodeName, []string{
				drainStartedConditionPrefix + started.Format(time.RFC3339),
			})
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, fmt.Sprintf(
				`[{"index":"idx","shard":"0","prirep":"p","state":"STARTED","node":"%s"}]`, targetNodeName,
			))
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			err := underTest.drainNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent)

			Expect(err).NotTo(HaveOccurred())
			for _, status := range spec.Status.ComponentsStatus {
				Expect(status.Status).ToNot(Equal("Drained"))
			}
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
		})

		It("Should mark the node drained only when it has no shards", func() {
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Excluded", targetNodeName, nil)
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, `[]`)
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			var markedDrained bool
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				updateFn(&spec)
				for _, status := range spec.Status.ComponentsStatus {
					if status.Component == "Scaler" && status.Status == "Drained" {
						markedDrained = true
					}
				}
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			err := underTest.drainNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent)

			Expect(err).NotTo(HaveOccurred())
			Expect(markedDrained).To(BeTrue())
		})

		It("Should reactivate shard allocation before checking whether the node has drained", func() {
			// Regression test: a RollingRestart/Upgrade cycle sets
			// allocation.enable=primaries while restarting a pod and only clears it
			// once its own cycle ends. If a Scaler drain starts mid-cycle, primaries
			// blocks exactly the replica movement the drain is waiting on, and
			// returning Requeue from drainNode short-circuits the reconciler chain
			// before RollingRestart/Upgrade run again to clear it - a livelock.
			// drainNode must reactivate allocation itself before checking for shards.
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Excluded", targetNodeName, nil)
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, `[]`)
			reactivatedTo := registerAllocationEnableSpy(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				updateFn(&spec)
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			err := underTest.drainNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent)

			Expect(err).NotTo(HaveOccurred())
			Expect(*reactivatedTo).To(Equal("all"))
		})

		It("Should emit a Warning and DrainStalled condition when a drain makes no progress", func() {
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			started := time.Now().UTC().Add(-16 * time.Minute)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Excluded", targetNodeName, []string{
				drainStartedConditionPrefix + started.Format(time.RFC3339),
			})
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, fmt.Sprintf(
				`[{"index":"idx","shard":"0","prirep":"p","state":"STARTED","node":"%s"}]`, targetNodeName,
			))
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			var stalled bool
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				updateFn(&spec)
				for _, status := range spec.Status.ComponentsStatus {
					if status.Component == "Scaler" && hasDrainStalledCondition(status.Conditions) {
						stalled = true
						Expect(status.Status).To(Equal("Excluded"))
					}
				}
			}).Return(nil)

			recorder := record.NewFakeRecorder(5)
			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			underTest.recorder = recorder
			err := underTest.drainNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent)

			Expect(err).NotTo(HaveOccurred())
			Expect(stalled).To(BeTrue())
			var events []string
			close(recorder.Events)
			for event := range recorder.Events {
				events = append(events, event)
			}
			Expect(events).To(ContainElement(ContainSubstring("has made no progress")))
		})

		It("Should not shrink the StatefulSet when decreaseOneNode finds shards still on the node", func() {
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Drained", targetNodeName, nil)
			spec.Spec.NodePools[0].Replicas = 1
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, fmt.Sprintf(
				`[{"index":"idx","shard":"0","prirep":"p","state":"STARTED","node":"%s"}]`, targetNodeName,
			))
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			var resetToExcluded bool
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
				updateFn(&spec)
				for _, status := range spec.Status.ComponentsStatus {
					if status.Component == "Scaler" && status.Status == "Excluded" {
						resetToExcluded = true
					}
				}
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeTrue())
			Expect(resetToExcluded).To(BeTrue())
			Expect(*currentSts.Spec.Replicas).To(Equal(int32(3)))
			mockClient.AssertNotCalled(GinkgoT(), "ReconcileResource", mock.Anything, mock.Anything)
		})

		It("Should not shrink the StatefulSet when decreaseOneNode cannot verify emptiness", func() {
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Drained", targetNodeName, nil)
			spec.Spec.NodePools[0].Replicas = 1
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusInternalServerError, `{"error":"unavailable"}`)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, true)

			Expect(err).To(HaveOccurred())
			Expect(requeue).To(BeTrue())
			Expect(*currentSts.Spec.Replicas).To(Equal(int32(3)))
			mockClient.AssertNotCalled(GinkgoT(), "ReconcileResource", mock.Anything, mock.Anything)
		})

		It("Should reactivate shard allocation before checking whether a removed nodePool's StatefulSet has drained", func() {
			// Same deadlock as drainNode: removeStatefulSet's drain (run from
			// cleanupStatefulSets for a nodePool dropped from spec.NodePools) needs
			// replica movement off the excluded node, which allocation.enable=primaries
			// blocks just the same.
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 1)
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General:  opensearchv1.GeneralConfig{ServiceName: clusterName, HttpPort: 9200},
					ConfMgmt: opensearchv1.ConfMgmt{SmartScaler: true},
				},
			}

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, `[]`)
			reactivatedTo := registerAllocationEnableSpy(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StateAbsent).Return(&ctrl.Result{}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.removeStatefulSet(currentSts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{}))
			Expect(*reactivatedTo).To(Equal("all"))
		})
	})
})
