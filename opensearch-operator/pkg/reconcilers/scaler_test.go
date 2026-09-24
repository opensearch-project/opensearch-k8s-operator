package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func recordVotingConfigCalls(transport *httpmock.MockTransport, postStatus, deleteStatus int) *[]string {
	calls := &[]string{}
	record := func(status int) func(*http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			*calls = append(*calls, req.Method+" "+req.URL.RawQuery)
			return httpmock.NewStringResponse(status, `{}`), nil
		}
	}
	transport.RegisterResponder(http.MethodPost, `=~.*/_cluster/voting_config_exclusions.*`, record(postStatus))
	transport.RegisterResponder(http.MethodDelete, `=~.*/_cluster/voting_config_exclusions.*`, record(deleteStatus))
	return calls
}

// registerCatNodesResponder serves _cat/nodes with the given node names as members.
func registerCatNodesResponder(transport *httpmock.MockTransport, names ...string) {
	registerCatNodesSequence(transport, names)
}

// registerCatNodesSequence serves _cat/nodes with each member list in turn; the
// last one repeats for all further calls.
func registerCatNodesSequence(transport *httpmock.MockTransport, sequence ...[]string) {
	call := 0
	transport.RegisterResponder(http.MethodGet, `=~.*/_cat/nodes.*`, func(req *http.Request) (*http.Response, error) {
		idx := call
		if idx >= len(sequence) {
			idx = len(sequence) - 1
		}
		call++
		entries := make([]string, 0, len(sequence[idx]))
		for _, n := range sequence[idx] {
			entries = append(entries, fmt.Sprintf(`{"name":%q,"node.role":"m","master":"-"}`, n))
		}
		return httpmock.NewStringResponse(200, "["+strings.Join(entries, ",")+"]"), nil
	})
}

// registerVotingExclusionsState serves the filtered cluster state with the given
// node names on the voting-config exclusions list.
func registerVotingExclusionsState(transport *httpmock.MockTransport, names ...string) {
	entries := make([]string, 0, len(names))
	for _, n := range names {
		entries = append(entries, fmt.Sprintf(`{"node_id":"id-%s","node_name":%q}`, n, n))
	}
	body := `{}`
	if len(entries) > 0 {
		body = `{"metadata":{"cluster_coordination":{"voting_config_exclusions":[` + strings.Join(entries, ",") + `]}}}`
	}
	transport.RegisterResponder(http.MethodGet, `=~.*/_cluster/state/metadata.*`, httpmock.NewStringResponder(200, body))
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
			ReadyReplicas:     replicas,
			AvailableReplicas: replicas,
		},
	}
}

// scalerTestPod builds a running pod of a node pool that reports the given readiness.
func scalerTestPod(clusterName, namespace, nodePoolComponent string, ordinal int, ready bool) corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%d", clusterName, nodePoolComponent, ordinal),
			Namespace: namespace,
			Labels: map[string]string{
				helpers.ClusterLabel:  clusterName,
				helpers.NodePoolLabel: nodePoolComponent,
			},
		},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
		},
	}
}

// listPodsOfNodePool matches the ListPods call helpers.ListPodsForNodePool makes for one node pool.
func listPodsOfNodePool(clusterName, nodePoolComponent string) interface{} {
	return mock.MatchedBy(func(opts *client.ListOptions) bool {
		return opts.LabelSelector != nil && opts.LabelSelector.Matches(labels.Set{
			helpers.ClusterLabel:  clusterName,
			helpers.NodePoolLabel: nodePoolComponent,
		})
	})
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

	Context("When a node pool is not fully available", func() {
		It("Should skip cleanup without short-circuiting the reconciler chain (issue #1531)", func() {
			// Requeue=true here would stop the main chain before the upgrade and
			// rolling-restart reconcilers run, so a stuck pod could never be recovered.
			clusterName := "test-cluster"
			clusterNamespace := "test-namespace"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				Spec: opensearchv1.ClusterSpec{
					General:   opensearchv1.GeneralConfig{Version: "2.12.0"},
					NodePools: []opensearchv1.NodePool{{Component: "masters", Replicas: 3}},
				},
				Status: opensearchv1.ClusterStatus{Version: "2.11.0"},
			}
			mastersSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-masters", Namespace: clusterNamespace},
				Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To[int32](3)},
				Status:     appsv1.StatefulSetStatus{AvailableReplicas: 2},
			}

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.On("GetStatefulSet", clusterName+"-masters", clusterNamespace).Return(mastersSts, nil)
			// No ListStatefulSets expectation: cleanup must be skipped.

			underTest := newScalerReconciler(mockClient, &spec)
			result, err := underTest.Reconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(drainPollInterval))
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

		It("Should requeue with a short fixed interval instead of exponential backoff while still draining (issue #1533)", func() {
			// A bare Requeue=true with no RequeueAfter is treated by controller-runtime
			// as a rate-limited re-add on the default exponential backoff limiter (5ms
			// doubling, capped at 1000s), which can delay noticing a stalled or completed
			// drain by many minutes. The still-draining case must set RequeueAfter instead.
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			started := time.Now().UTC().Add(-time.Minute)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Excluded", targetNodeName, []string{
				drainStartedConditionPrefix + started.Format(time.RFC3339),
			})
			currentSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)
			currentSts.Labels = map[string]string{
				helpers.ClusterLabel:  clusterName,
				helpers.NodePoolLabel: nodePoolComponent,
			}

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, fmt.Sprintf(
				`[{"index":"idx","shard":"0","prirep":"p","state":"STARTED","node":"%s"}]`, targetNodeName,
			))
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("GetStatefulSet", clusterName+"-"+nodePoolComponent, clusterNamespace).Return(currentSts, nil)
			// All pods are up: the drain only progresses while no pod of any pool is down (issue #1590).
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 0, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 1, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 2, true),
			}}, nil)
			mockClient.On("ListStatefulSets",
				client.InNamespace(clusterNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{currentSts},
			}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(drainPollInterval))
			mockClient.AssertExpectations(GinkgoT())
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
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, true, false)

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
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, true, false)

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

	Context("When removing master-eligible nodes (issue #1448)", func() {
		const (
			clusterName       = "test-cluster"
			clusterNamespace  = "test-namespace"
			nodePoolComponent = "masters"
		)

		masterDecreaseCluster := func(replicas int32) (opensearchv1.OpenSearchCluster, appsv1.StatefulSet, string) {
			stsName := fmt.Sprintf("%s-%s", clusterName, nodePoolComponent)
			targetNodeName := fmt.Sprintf("%s-%d", stsName, replicas-1)
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General:  opensearchv1.GeneralConfig{ServiceName: clusterName, HttpPort: 9200},
					ConfMgmt: opensearchv1.ConfMgmt{SmartScaler: false},
					NodePools: []opensearchv1.NodePool{
						{Component: nodePoolComponent, Replicas: replicas - 1, Roles: []string{"cluster_manager"}},
					},
				},
				Status: opensearchv1.ClusterStatus{
					Initialized: true,
					ComponentsStatus: []opensearchv1.ComponentStatus{
						{Component: "Scaler", Status: "Drained", Description: nodePoolComponent, Conditions: []string{targetNodeName}},
					},
				},
			}
			sts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName,
					Namespace: clusterNamespace,
					Labels:    map[string]string{"opensearch.role": "cluster_manager"},
				},
				Spec:   appsv1.StatefulSetSpec{Replicas: ptr.To(replicas)},
				Status: appsv1.StatefulSetStatus{ReadyReplicas: replicas},
			}
			return spec, sts, targetNodeName
		}

		newTransport := func(spec *opensearchv1.OpenSearchCluster) *httpmock.MockTransport {
			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, spec)
			registerClusterSettingsResponders(transport)
			return transport
		}

		It("Should POST the exclusion, shrink, and defer the clear while the removed node is still a member", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			transport := newTransport(&spec)
			registerCatNodesResponder(transport, targetNodeName)
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StatePresent).Return(&ctrl.Result{}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, false, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeTrue())
			Expect(*currentSts.Spec.Replicas).To(Equal(int32(2)))
			// The exclusion is re-applied right before shrinking; the waiting clear is
			// not issued while the node is still in the cluster (it would time out).
			Expect(*calls).To(Equal([]string{"POST node_names=" + targetNodeName + "&timeout=10s"}))
			Expect(spec.Status.ComponentsStatus).To(HaveLen(1))
			Expect(spec.Status.ComponentsStatus[0].Status).To(Equal("Drained"))
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
		})

		It("Should clear with wait and drop the status once the removed node has left the cluster", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			transport := newTransport(&spec)
			registerCatNodesSequence(transport, []string{targetNodeName}, []string{})
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StatePresent).Return(&ctrl.Result{}, nil)
			var statusRemoved bool
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(&spec)
				statusRemoved = len(spec.Status.ComponentsStatus) == 0
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, false, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeFalse())
			Expect(statusRemoved).To(BeTrue())
			Expect(*calls).To(Equal([]string{
				"POST node_names=" + targetNodeName + "&timeout=10s",
				"DELETE wait_for_removal=true",
			}))
		})

		It("Should keep scaler status and not clear without wait if the waiting DELETE fails", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			transport := newTransport(&spec)
			registerCatNodesSequence(transport, []string{targetNodeName}, []string{})
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusInternalServerError)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StatePresent).Return(&ctrl.Result{}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			currentStatus := spec.Status.ComponentsStatus[0]
			requeue, err := underTest.decreaseOneNode(currentStatus, currentSts, nodePoolComponent, false, true)

			Expect(err).To(HaveOccurred())
			Expect(requeue).To(BeTrue())
			Expect(spec.Status.ComponentsStatus).To(HaveLen(1))
			Expect(spec.Status.ComponentsStatus[0].Status).To(Equal("Drained"))
			Expect(*calls).To(Equal([]string{
				"POST node_names=" + targetNodeName + "&timeout=10s",
				"DELETE wait_for_removal=true",
			}))
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
		})

		It("Should skip the POST for a node that is not a cluster member instead of recording an _absent_ tombstone", func() {
			spec, currentSts, _ := masterDecreaseCluster(3)
			transport := newTransport(&spec)
			registerCatNodesResponder(transport) // target already gone
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StatePresent).Return(&ctrl.Result{}, nil)
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(&spec)
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			requeue, err := underTest.decreaseOneNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, false, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeFalse())
			Expect(*currentSts.Spec.Replicas).To(Equal(int32(2)))
			Expect(*calls).To(Equal([]string{"DELETE wait_for_removal=true"}))
		})

		reconcileMasterPool := func(spec *opensearchv1.OpenSearchCluster, currentSts appsv1.StatefulSet, members ...string) (bool, *[]string, error) {
			transport := newTransport(spec)
			registerCatNodesResponder(transport, members...)
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("GetStatefulSet", currentSts.Name, clusterNamespace).Return(currentSts, nil)
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{}, nil).Maybe()
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(spec)
			}).Return(nil).Maybe()

			underTest := newScalerReconciler(mockClient, spec)
			underTest.osClientTransport = transport
			requeue, err := underTest.reconcileNodePool(&spec.Spec.NodePools[0])
			mockClient.AssertNotCalled(GinkgoT(), "ReconcileResource", mock.Anything, mock.Anything)
			return requeue, calls, err
		}

		It("Should clear voting exclusions without wait when a master scale-down is reverted before removal", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			spec.Spec.NodePools[0].Replicas = 3 // reverted: target is still a live member of the pool

			requeue, calls, err := reconcileMasterPool(&spec, currentSts, targetNodeName)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeFalse())
			Expect(spec.Status.ComponentsStatus).To(BeEmpty())
			// A waiting clear would never finish, since the node is not leaving.
			Expect(*calls).To(Equal([]string{"DELETE wait_for_removal=false"}))
		})

		It("Should treat a legacy Drained status without a target as a reverted scale-down", func() {
			spec, currentSts, _ := masterDecreaseCluster(3)
			spec.Spec.NodePools[0].Replicas = 3
			spec.Status.ComponentsStatus[0].Conditions = nil

			requeue, calls, err := reconcileMasterPool(&spec, currentSts)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeFalse())
			Expect(spec.Status.ComponentsStatus).To(BeEmpty())
			Expect(*calls).To(Equal([]string{"DELETE wait_for_removal=false"}))
		})

		It("Should keep waiting for a removed node to leave before finishing pending cleanup", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			currentSts.Spec.Replicas = ptr.To[int32](2) // decreaseOneNode shrank it, the node is still terminating

			requeue, calls, err := reconcileMasterPool(&spec, currentSts, targetNodeName)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeTrue())
			Expect(spec.Status.ComponentsStatus).To(HaveLen(1))
			Expect(*calls).To(BeEmpty())
		})

		It("Should finish pending cleanup with a waiting clear once the removed node has left", func() {
			spec, currentSts, _ := masterDecreaseCluster(3)
			currentSts.Spec.Replicas = ptr.To[int32](2)

			requeue, calls, err := reconcileMasterPool(&spec, currentSts)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeFalse())
			Expect(spec.Status.ComponentsStatus).To(BeEmpty())
			Expect(*calls).To(Equal([]string{"DELETE wait_for_removal=true"}))
		})

		It("Should POST a voting-config exclusion on master scale-down and skip the allocation exclude with SmartScaler off", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			spec.Status.ComponentsStatus[0].Status = "Running"
			spec.Status.ComponentsStatus[0].Conditions = nil
			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatNodesResponder(transport, targetNodeName)
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(&spec)
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			err := underTest.excludeNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent, true)

			// No _cluster/settings responder is registered: an allocation exclude would 404.
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Status.ComponentsStatus[0].Status).To(Equal("Excluded"))
			Expect(*calls).To(Equal([]string{"POST node_names=" + targetNodeName + "&timeout=10s"}))
		})

		It("Should mark a master node Drained without a shard drain when SmartScaler is off", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(3)
			spec.Status.ComponentsStatus[0].Status = "Excluded"

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(&spec)
			}).Return(nil)

			underTest := newScalerReconciler(mockClient, &spec)
			// No transport at all: any OpenSearch call (allocation reactivate, _cat/shards) would fail.
			err := underTest.drainNode(spec.Status.ComponentsStatus[0], currentSts, nodePoolComponent)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Status.ComponentsStatus[0].Status).To(Equal("Drained"))
			Expect(spec.Status.ComponentsStatus[0].Conditions).To(Equal([]string{targetNodeName}))
		})

		It("Should POST then delete the StatefulSet and leave the clear to the sweep while the node is still a member", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(1)
			spec.Status.ComponentsStatus = nil
			currentSts.Spec.Replicas = ptr.To[int32](1)
			transport := newTransport(&spec)
			registerCatNodesResponder(transport, targetNodeName)
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StateAbsent).Return(&ctrl.Result{}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.removeStatefulSet(currentSts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{}))
			Expect(*calls).To(Equal([]string{"POST node_names=" + targetNodeName + "&timeout=10s"}))
		})

		It("Should POST then waiting-DELETE voting exclusions when removing a master StatefulSet whose node has left", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(1)
			spec.Status.ComponentsStatus = nil
			currentSts.Spec.Replicas = ptr.To[int32](1)
			transport := newTransport(&spec)
			registerCatNodesSequence(transport, []string{targetNodeName}, []string{})
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StateAbsent).Return(&ctrl.Result{}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.removeStatefulSet(currentSts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&ctrl.Result{}))
			Expect(*calls).To(Equal([]string{
				"POST node_names=" + targetNodeName + "&timeout=10s",
				"DELETE wait_for_removal=true",
			}))
		})

		It("Should detect a master StatefulSet by its node.roles env when the role label is missing", func() {
			spec, currentSts, targetNodeName := masterDecreaseCluster(1)
			spec.Status.ComponentsStatus = nil
			currentSts.Spec.Replicas = ptr.To[int32](1)
			currentSts.Labels = map[string]string{"opensearch.role": "custom"}
			currentSts.Spec.Template.Spec.Containers = []corev1.Container{{
				Name: "opensearch",
				Env:  []corev1.EnvVar{{Name: "node.roles", Value: "cluster_manager,data"}},
			}}
			transport := newTransport(&spec)
			registerCatNodesResponder(transport, targetNodeName)
			calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StateAbsent).Return(&ctrl.Result{}, nil)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			_, err := underTest.removeStatefulSet(currentSts)

			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(Equal([]string{"POST node_names=" + targetNodeName + "&timeout=10s"}))
		})

		Context("voting-config exclusion sweep", func() {
			sweep := func(spec *opensearchv1.OpenSearchCluster, excluded, members []string, setup func(*k8s.MockK8sClient)) *[]string {
				transport := newTransport(spec)
				registerCatNodesResponder(transport, members...)
				registerVotingExclusionsState(transport, excluded...)
				calls := recordVotingConfigCalls(transport, http.StatusOK, http.StatusOK)

				mockClient := k8s.NewMockK8sClient(GinkgoT())
				mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
				if setup != nil {
					setup(mockClient)
				}
				underTest := newScalerReconciler(mockClient, spec)
				underTest.osClientTransport = transport
				underTest.sweepVotingConfigExclusions()
				return calls
			}

			It("Should do nothing when there are no exclusions", func() {
				spec, _, _ := masterDecreaseCluster(3)
				spec.Status.ComponentsStatus = nil
				calls := sweep(&spec, nil, []string{"test-cluster-masters-0"}, nil)
				Expect(*calls).To(BeEmpty())
			})

			It("Should clear with wait once every excluded node has left the cluster", func() {
				spec, _, targetNodeName := masterDecreaseCluster(3)
				spec.Status.ComponentsStatus = nil
				calls := sweep(&spec, []string{targetNodeName}, []string{"test-cluster-masters-0"}, nil)
				Expect(*calls).To(Equal([]string{"DELETE wait_for_removal=true"}))
			})

			It("Should leave an exclusion alone while a scale-down still targets that node", func() {
				spec, _, targetNodeName := masterDecreaseCluster(3)
				calls := sweep(&spec, []string{targetNodeName}, []string{targetNodeName}, nil)
				Expect(*calls).To(BeEmpty())
			})

			It("Should leave the bootstrap node's exclusion alone while it is still a member", func() {
				spec, _, _ := masterDecreaseCluster(3)
				spec.Status.ComponentsStatus = nil
				bootstrap := builders.BootstrapPodName(&spec)
				calls := sweep(&spec, []string{bootstrap}, []string{bootstrap}, nil)
				Expect(*calls).To(BeEmpty())
			})

			It("Should leave an exclusion alone while its pod is terminating", func() {
				spec, _, targetNodeName := masterDecreaseCluster(3)
				spec.Status.ComponentsStatus = nil
				calls := sweep(&spec, []string{targetNodeName}, []string{targetNodeName}, func(m *k8s.MockK8sClient) {
					now := metav1.Now()
					m.On("GetPod", targetNodeName, clusterNamespace).Return(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: targetNodeName, DeletionTimestamp: &now}}, nil)
				})
				Expect(*calls).To(BeEmpty())
			})

			It("Should clear without wait when a live pool member nothing is removing is still excluded", func() {
				spec, currentSts, targetNodeName := masterDecreaseCluster(3)
				spec.Status.ComponentsStatus = nil
				spec.Spec.NodePools[0].Replicas = 3
				calls := sweep(&spec, []string{targetNodeName}, []string{targetNodeName}, func(m *k8s.MockK8sClient) {
					m.On("GetPod", targetNodeName, clusterNamespace).Return(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: targetNodeName}}, nil)
					m.On("GetStatefulSet", currentSts.Name, clusterNamespace).Return(currentSts, nil)
				})
				Expect(*calls).To(Equal([]string{"DELETE wait_for_removal=false"}))
			})

			It("Should not clear without wait while another excluded node is still leaving", func() {
				spec, currentSts, targetNodeName := masterDecreaseCluster(3)
				spec.Status.ComponentsStatus = nil
				spec.Spec.NodePools[0].Replicas = 3
				leaving := "test-cluster-old-0"
				calls := sweep(&spec, []string{targetNodeName, leaving}, []string{targetNodeName, leaving}, func(m *k8s.MockK8sClient) {
					m.On("GetPod", targetNodeName, clusterNamespace).Return(corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: targetNodeName}}, nil)
					m.On("GetStatefulSet", currentSts.Name, clusterNamespace).Return(currentSts, nil)
					m.On("GetPod", leaving, clusterNamespace).Return(corev1.Pod{}, k8serrors.NewNotFound(schema.GroupResource{Resource: "pods"}, leaving))
				})
				Expect(*calls).To(BeEmpty())
			})

			It("Should skip the sweep entirely before the cluster is initialized", func() {
				spec, _, _ := masterDecreaseCluster(3)
				spec.Status.Initialized = false
				// No admin secret is mocked and no transport is set: any client creation would fail the mock.
				mockClient := k8s.NewMockK8sClient(GinkgoT())
				underTest := newScalerReconciler(mockClient, &spec)
				underTest.sweepVotingConfigExclusions()
			})
		})
	})

	Context("When a pod is down during a scale-down (issue #1590)", func() {
		const (
			clusterName       = "test-cluster"
			clusterNamespace  = "test-namespace"
			nodePoolComponent = "data"
		)
		stsName := fmt.Sprintf("%s-%s", clusterName, nodePoolComponent)

		// stsWithOnePodDown is a StatefulSet whose status reports one of its pods missing.
		stsWithOnePodDown := func(name string, replicas int32) appsv1.StatefulSet {
			return appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: clusterNamespace},
				Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(replicas)},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: replicas - 1, AvailableReplicas: replicas - 1},
			}
		}
		// spyOnStsWrite records the replica count of any StatefulSet the scaler writes.
		spyOnStsWrite := func(mockClient *k8s.MockK8sClient) *int32 {
			written := ptr.To[int32](-1)
			mockClient.On("ReconcileResource", mock.Anything, reconciler.StatePresent).Run(func(args mock.Arguments) {
				if sts, ok := args.Get(0).(*appsv1.StatefulSet); ok {
					*written = ptr.Deref(sts.Spec.Replicas, -1)
				}
			}).Return(&ctrl.Result{}, nil).Maybe()
			return written
		}
		// spyOnExclusions counts PUT _cluster/settings calls, which is how excludeNode starts a drain.
		spyOnExclusions := func(transport *httpmock.MockTransport) *int {
			puts := new(int)
			transport.RegisterResponder(http.MethodPut, `=~.*/_cluster/settings.*`, func(*http.Request) (*http.Response, error) {
				*puts++
				return httpmock.NewStringResponse(200, `{"transient":{},"persistent":{}}`), nil
			})
			return puts
		}
		// allowStatusUpdates lets the scaler write its component status so a failure
		// shows up on the behavioural assertion rather than as an unexpected mock call.
		allowStatusUpdates := func(mockClient *k8s.MockK8sClient, spec *opensearchv1.OpenSearchCluster) {
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(spec)
			}).Return(nil).Maybe()
		}

		It("Should not remove a drained node while another pod of the pool is down", func() {
			// Reverse direction of #1572: the rolling restart deleted data-0 and it is
			// not back yet, then the user lowered replicas. The scaler runs before the
			// restart in the chain, so without a gate of its own it finishes the drain
			// and shrinks the StatefulSet - two members are out of the cluster at once.
			targetNodeName := fmt.Sprintf("%s-%s-2", clusterName, nodePoolComponent)
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "Drained", targetNodeName, nil)
			spec.Spec.General.Version = "2.11.0"
			spec.Status.Version = "2.11.0"
			spec.Status.ComponentsStatus = append(spec.Status.ComponentsStatus, opensearchv1.ComponentStatus{
				Component: componentName,
				Status:    statusInProgress,
			})
			// data-0 was deleted by the rolling restart and has not come back
			currentSts := stsWithOnePodDown(stsName, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerCatShardsResponder(transport, http.StatusOK, `[]`) // target node already empty
			registerClusterSettingsResponders(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(currentSts, nil)
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 0, false),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 1, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 2, true),
			}}, nil)
			// Only reached once the pool looks settled again; nothing to clean up.
			mockClient.On("ListStatefulSets",
				client.InNamespace(clusterNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{}, nil).Maybe()
			allowStatusUpdates(mockClient, &spec)
			shrunkTo := spyOnStsWrite(mockClient)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(*shrunkTo).To(Equal(int32(-1)), "scaler shrank the StatefulSet while another pod of the pool was down")
			// Must not short-circuit the chain either: the rolling restart runs after the
			// scaler and is the only thing that can finish the restart and make the pool
			// ready again, so Requeue=true here would deadlock the two reconcilers.
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})

		It("Should not start a drain while a pod of the pool is not ready", func() {
			// replicas lowered 3 -> 2, no scaling in progress yet, smartScaler on
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "", "", nil)
			spec.Status.ComponentsStatus = nil
			currentSts := stsWithOnePodDown(stsName, 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerClusterSettingsResponders(transport)
			exclusionPuts := spyOnExclusions(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(currentSts, nil)
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 0, false),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 1, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 2, true),
			}}, nil)
			allowStatusUpdates(mockClient, &spec)
			shrunkTo := spyOnStsWrite(mockClient)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(*exclusionPuts).To(Equal(0), "scaler excluded a node while a pod of the pool was down")
			Expect(*shrunkTo).To(Equal(int32(-1)))
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(drainPollInterval))
		})

		It("Should not remove a node without draining while a pod of the pool is not ready", func() {
			// Same as above with smartScaler off, where the removal would be immediate.
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "", "", nil)
			spec.Status.ComponentsStatus = nil
			spec.Spec.ConfMgmt.SmartScaler = false
			currentSts := stsWithOnePodDown(stsName, 3)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(currentSts, nil)
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 0, false),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 1, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 2, true),
			}}, nil)
			// Only reached once the pool looks settled again; nothing to clean up.
			mockClient.On("ListStatefulSets",
				client.InNamespace(clusterNamespace),
				client.MatchingLabels{helpers.ClusterLabel: clusterName}).Return(appsv1.StatefulSetList{}, nil).Maybe()
			allowStatusUpdates(mockClient, &spec)
			shrunkTo := spyOnStsWrite(mockClient)

			underTest := newScalerReconciler(mockClient, &spec)
			result, err := underTest.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(*shrunkTo).To(Equal(int32(-1)), "scaler removed a node while a pod of the pool was down")
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(drainPollInterval))
		})

		It("Should hold a scale-down while a pod of another node pool is not ready", func() {
			// The gate is cluster-wide: a member down anywhere is one member down.
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "", "", nil)
			spec.Status.ComponentsStatus = nil
			spec.Spec.NodePools = append(spec.Spec.NodePools, opensearchv1.NodePool{Component: "masters", Replicas: 3})
			dataSts := scalerDrainTestSts(clusterName, clusterNamespace, nodePoolComponent, 3)
			mastersSts := stsWithOnePodDown(clusterName+"-masters", 3)

			transport := httpmock.NewMockTransport()
			transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
			registerOsPingResponders(transport, &spec)
			registerClusterSettingsResponders(transport)
			exclusionPuts := spyOnExclusions(transport)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockScalerAdminSecret(mockClient, clusterName, clusterNamespace)
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(dataSts, nil)
			mockClient.On("GetStatefulSet", clusterName+"-masters", clusterNamespace).Return(mastersSts, nil)
			mockClient.On("ListPods", listPodsOfNodePool(clusterName, nodePoolComponent)).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 0, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 1, true),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 2, true),
			}}, nil)
			mockClient.On("ListPods", listPodsOfNodePool(clusterName, "masters")).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, "masters", 0, true),
				scalerTestPod(clusterName, clusterNamespace, "masters", 1, false),
				scalerTestPod(clusterName, clusterNamespace, "masters", 2, true),
			}}, nil)
			allowStatusUpdates(mockClient, &spec)
			shrunkTo := spyOnStsWrite(mockClient)

			underTest := newScalerReconciler(mockClient, &spec)
			underTest.osClientTransport = transport
			result, err := underTest.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(*exclusionPuts).To(Equal(0), "scaler excluded a data node while a master pod was down")
			Expect(*shrunkTo).To(Equal(int32(-1)))
			mockClient.AssertNotCalled(GinkgoT(), "UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything)
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(drainPollInterval))
		})

		It("Should still scale up while a pod of the pool is not ready", func() {
			// Adding capacity never takes a member out and is a way to recover a
			// degraded cluster, so it must not wait on readiness.
			spec := scalerDrainTestCluster(clusterName, clusterNamespace, nodePoolComponent, "", "", nil)
			spec.Status.ComponentsStatus = nil
			spec.Spec.ConfMgmt.SmartScaler = false
			spec.Spec.NodePools[0].Replicas = 3
			currentSts := stsWithOnePodDown(stsName, 2)

			mockClient := k8s.NewMockK8sClient(GinkgoT())
			mockClient.On("GetStatefulSet", stsName, clusterNamespace).Return(currentSts, nil)
			mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{Items: []corev1.Pod{
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 0, false),
				scalerTestPod(clusterName, clusterNamespace, nodePoolComponent, 1, true),
			}}, nil)
			mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(&spec), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
				args.Get(1).(func(*opensearchv1.OpenSearchCluster))(&spec)
			}).Return(nil)
			grownTo := spyOnStsWrite(mockClient)

			underTest := newScalerReconciler(mockClient, &spec)
			result, err := underTest.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(*grownTo).To(Equal(int32(3)), "scale-up did not proceed while a pod was down")
			Expect(result.Requeue).To(BeFalse())
			mockClient.AssertExpectations(GinkgoT())
		})
	})
})
