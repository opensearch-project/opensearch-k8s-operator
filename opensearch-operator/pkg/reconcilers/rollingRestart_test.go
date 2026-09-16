package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("RollingRestart readiness gate during a scale-down", func() {
	// The scaler has just lowered the StatefulSet from 3 to 2 replicas, so data-2 is
	// terminating, while a config change left data-0/data-1 on an older revision. The
	// restart must not start (and delete data-0) until data-2 is actually gone,
	// otherwise two members leave at once (issue #1572).
	It("must not start a rolling restart while a pod removed by the scaler is still terminating", func() {
		clusterName := "test-cluster"
		clusterNamespace := "test-namespace"
		cluster := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace, UID: "dummyuid"},
			Spec: opensearchv1.ClusterSpec{
				General:   opensearchv1.GeneralConfig{Version: "2.11.0", ServiceName: clusterName, HttpPort: 9200},
				NodePools: []opensearchv1.NodePool{{Component: "data", Replicas: 2, Roles: []string{"data"}}},
			},
			Status: opensearchv1.ClusterStatus{
				Version:          "2.11.0",
				Initialized:      true,
				ComponentsStatus: []opensearchv1.ComponentStatus{}, // Scaler entry already removed by decreaseOneNode
			},
		}
		sts := appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName + "-data", Namespace: clusterNamespace},
			Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To[int32](2)}, // already lowered by the scaler
			Status: appsv1.StatefulSetStatus{
				CurrentRevision: "rev1",
				UpdateRevision:  "rev2", // config change pending
				UpdatedReplicas: 0,
			},
		}
		readyPod := func(ordinal int, terminating bool) corev1.Pod {
			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-data-%d", clusterName, ordinal),
					Namespace: clusterNamespace,
					Labels: map[string]string{
						helpers.ClusterLabel:       clusterName,
						helpers.NodePoolLabel:      "data",
						"controller-revision-hash": "rev1",
					},
				},
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
			}
			if terminating {
				pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			}
			return pod
		}

		mockClient := k8s.NewMockK8sClient(GinkgoT())
		mockClient.On("GetStatefulSet", clusterName+"-data", clusterNamespace).Return(sts, nil)
		mockClient.On("ListPods", mock.Anything).Return(corev1.PodList{Items: []corev1.Pod{
			readyPod(0, false), readyPod(1, false), readyPod(2, true),
		}}, nil)
		mockClient.On("GetPod", mock.Anything, clusterNamespace).Return(readyPod(0, false), nil).Maybe()
		restartStarted := false
		mockClient.On("UpdateOpenSearchClusterStatus", client.ObjectKeyFromObject(cluster), mock.AnythingOfType("func(*v1.OpenSearchCluster)")).Run(func(args mock.Arguments) {
			args.Get(1).(func(*opensearchv1.OpenSearchCluster))(cluster)
			for _, status := range cluster.Status.ComponentsStatus {
				if status.Component == componentName && status.Status == statusInProgress {
					restartStarted = true
				}
			}
		}).Return(nil).Maybe()
		// Reached only if the gate lets the restart through; fail fast instead of dialing a cluster.
		mockClient.On("GetSecret", mock.Anything, clusterNamespace).Return(corev1.Secret{}, errors.New("restart must not need a cluster client here")).Maybe()

		reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, cluster, cluster.Spec.NodePools)
		underTest := &RollingRestartReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			recorder:          record.NewFakeRecorder(20),
			reconcilerContext: &reconcilerContext,
			instance:          cluster,
			logger:            zap.New().WithName("restart-test"),
		}
		result, err := underTest.Reconcile()

		Expect(err).NotTo(HaveOccurred())
		Expect(restartStarted).To(BeFalse(), "rolling restart started while a scaled-down pod was still terminating")
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))
		mockClient.AssertNotCalled(GinkgoT(), "DeletePod", mock.Anything)
	})
})

var _ = Describe("RollingRestart Reconciler", func() {
	Describe("hasManagerRole", func() {
		Context("with cluster_manager role", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "master",
					Roles:     []string{"cluster_manager"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with master role", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "master",
					Roles:     []string{"master"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with data role only", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "data",
					Roles:     []string{"data"},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeFalse())
			})
		})

		Context("with no roles", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "coordinating",
					Roles:     []string{},
				}
				Expect(helpers.HasManagerRole(&nodePool)).To(BeFalse())
			})
		})
	})

	Describe("hasDataRole", func() {
		Context("with data role", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "data",
					Roles:     []string{"data"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with cluster_manager role only", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "master",
					Roles:     []string{"cluster_manager"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeFalse())
			})
		})

		Context("with multiple roles including data", func() {
			It("should return true", func() {
				nodePool := opensearchv1.NodePool{
					Component: "coordinating",
					Roles:     []string{"data", "ingest"},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeTrue())
			})
		})

		Context("with no roles", func() {
			It("should return false", func() {
				nodePool := opensearchv1.NodePool{
					Component: "coordinating",
					Roles:     []string{},
				}
				Expect(helpers.HasDataRole(&nodePool)).To(BeFalse())
			})
		})
	})

	Describe("findStatus", func() {
		Context("when ComponentsStatus has a non-RollingRestart entry at index 0", func() {
			It("should return the matched RollingRestart entry, not comp[0]", func() {
				instance := &opensearchv1.OpenSearchCluster{
					Status: opensearchv1.ClusterStatus{
						ComponentsStatus: []opensearchv1.ComponentStatus{
							{Component: "Scaler", Status: statusInProgress},
							{Component: componentName, Status: statusInProgress},
						},
					},
				}
				r := &RollingRestartReconciler{instance: instance}

				found := r.findStatus()

				Expect(found).NotTo(BeNil())
				Expect(found.Component).To(Equal(componentName))
				Expect(found.Status).To(Equal(statusInProgress))
			})
		})

		Context("when there is no RollingRestart entry", func() {
			It("should return nil", func() {
				instance := &opensearchv1.OpenSearchCluster{
					Status: opensearchv1.ClusterStatus{
						ComponentsStatus: []opensearchv1.ComponentStatus{
							{Component: "Scaler", Status: statusInProgress},
						},
					},
				}
				r := &RollingRestartReconciler{instance: instance}

				Expect(r.findStatus()).To(BeNil())
			})
		})
	})
})
