package reconcilers

import (
	"context"
	"fmt"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Regression tests for issue #1471: claim is that after the operator deletes one
// cluster-manager pod, a reconcile running against a stale informer cache can pass
// the quorum guard and delete a *second* manager, breaking quorum.
//
// These tests freeze the "cache" (the mocked K8sClient) at each state it can be in
// after the first deletion and assert what the reconciler does. A real e2e test
// cannot deterministically hold the informer cache stale, so the mock IS the stale
// cache — strictly worse than reality, since Reconcile()'s outer readiness gate is
// bypassed too.
var _ = Describe("RollingRestart master quorum guard vs stale informer cache (issue #1471)", func() {
	const (
		ns     = "rr-race"
		revOld = "rr-race-masters-rev-old"
		revNew = "rr-race-masters-rev-new"
	)

	var (
		transport  *httpmock.MockTransport
		mockClient *k8s.MockK8sClient
		cluster    *opensearchv1.OpenSearchCluster
		reconciler *RollingRestartReconciler
		deleted    []string
		stsName    string
	)

	managerPod := func(ordinal int, revision string, ready bool, terminating bool) corev1.Pod {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", stsName, ordinal),
				Namespace: ns,
				Labels:    map[string]string{"controller-revision-hash": revision},
			},
		}
		if ready {
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		}
		if terminating {
			pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		}
		return pod
	}

	managerSts := func(updatedReplicas int32) appsv1.StatefulSet {
		return appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: stsName, Namespace: ns},
			Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(int32(3))},
			Status: appsv1.StatefulSetStatus{
				CurrentRevision: revOld,
				UpdateRevision:  revNew,
				UpdatedReplicas: updatedReplicas,
			},
		}
	}

	// setCacheState wires the mock to return one frozen snapshot for every read,
	// exactly like an informer cache that has not moved since the snapshot.
	setCacheState := func(sts appsv1.StatefulSet, pods []corev1.Pod) {
		mockClient.EXPECT().GetStatefulSet(stsName, ns).Return(sts, nil)
		for i := range pods {
			mockClient.EXPECT().GetPod(pods[i].Name, ns).Return(pods[i], nil).Maybe()
		}
		mockClient.EXPECT().ListPods(mock.Anything).Return(corev1.PodList{Items: pods}, nil)
	}

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))

		cluster = &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "rr-race", Namespace: ns},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					ServiceName: "rr-race",
					HttpPort:    9200,
					Version:     "2.19.1",
				},
				NodePools: []opensearchv1.NodePool{
					{
						Component: "masters",
						Replicas:  3,
						Roles:     []string{"cluster_manager"},
					},
				},
			},
			Status: opensearchv1.ClusterStatus{Initialized: true},
		}
		stsName = builders.StsName(cluster, &cluster.Spec.NodePools[0])

		clusterUrl := fmt.Sprintf("%s/", helpers.ClusterURL(cluster))
		transport.RegisterResponder("HEAD", clusterUrl, httpmock.NewStringResponder(200, ""))
		transport.RegisterResponder("GET", clusterUrl,
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"version": map[string]interface{}{"number": "2.19.1", "distribution": "opensearch"},
			}))
		transport.RegisterResponder("GET", clusterUrl+"_cluster/health",
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{"status": "green"}))
		transport.RegisterResponder("PUT", clusterUrl+"_cluster/settings",
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{}))

		osClient, err := services.NewOsClusterClient(clusterUrl, "admin", "admin", services.WithTransport(transport))
		Expect(err).NotTo(HaveOccurred())

		deleted = nil
		reconciler = &RollingRestartReconciler{
			client:   mockClient,
			ctx:      context.Background(),
			instance: cluster,
			logger:   log.FromContext(context.Background()),
			osClient: osClient,
			recorder: record.NewFakeRecorder(10),
		}
	})

	// State A — the window the issue describes: managers-0 was already deleted on the
	// API server, but the cache has not observed anything yet (all 3 pods present,
	// ready, old revision). The issue claims the reconciler now disrupts a *second*
	// manager. It cannot: candidate selection reads the same stale snapshot as the
	// quorum count, so it re-selects managers-0 — the pod that is already gone.
	When("the cache has not observed the first deletion at all", func() {
		It("re-targets the already-deleted pod, never a second manager", func() {
			pods := []corev1.Pod{
				managerPod(0, revOld, true, false),
				managerPod(1, revOld, true, false),
				managerPod(2, revOld, true, false),
			}
			setCacheState(managerSts(0), pods)
			mockClient.EXPECT().DeletePod(mock.Anything).RunAndReturn(func(pod *corev1.Pod) error {
				deleted = append(deleted, pod.Name)
				return nil
			}).Once()

			result, err := reconciler.globalCandidateRollingRestart()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			// The quorum guard passes (it sees 3 ready), but the delete goes to the
			// same ordinal that was already deleted — not to a different manager.
			Expect(deleted).To(Equal([]string{stsName + "-0"}))
		})
	})

	// State B — the cache observed the deletion: managers-0 shows as terminating.
	// The ready count drops to 2 == minRequired and the guard refuses to act.
	When("the cache shows the first pod terminating", func() {
		It("refuses to disrupt another manager and requeues", func() {
			pods := []corev1.Pod{
				managerPod(0, revOld, true, true),
				managerPod(1, revOld, true, false),
				managerPod(2, revOld, true, false),
			}
			setCacheState(managerSts(0), pods)
			// No DeletePod expectation: any deletion fails the test.

			result, err := reconciler.globalCandidateRollingRestart()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(deleted).To(BeEmpty())
		})
	})

	// State C — the cache observed deletion and recreation: managers-0 is back at the
	// new revision but not yet ready. This is the only state in which managers-1
	// becomes the restart candidate (as in the issue's log), and here the ready count
	// is necessarily 2, so the guard trips. For the issue's logged combination
	// (candidate=managers-1 AND readyMasters=3) the replacement would have to be
	// genuinely Ready — in which case deleting managers-1 is safe.
	When("the cache shows the replacement pod running but not ready", func() {
		It("selects the next manager as candidate but the quorum guard blocks it", func() {
			pods := []corev1.Pod{
				managerPod(0, revNew, false, false),
				managerPod(1, revOld, true, false),
				managerPod(2, revOld, true, false),
			}
			setCacheState(managerSts(1), pods)
			// No DeletePod expectation: any deletion fails the test.

			result, err := reconciler.globalCandidateRollingRestart()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(deleted).To(BeEmpty())
		})
	})
})
