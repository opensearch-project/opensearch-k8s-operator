package reconcilers

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("Upgrade version validation", func() {
	It("returns a terminal error on downgrade so the chain can continue", func() {
		spec := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{Version: "2.10.0"},
			},
			Status: opensearchv1.ClusterStatus{
				Initialized: true,
				Version:     "2.11.0",
			},
		}
		mockClient := k8s.NewMockK8sClient(GinkgoT())
		ctx := NewReconcilerContext(&helpers.MockEventRecorder{}, spec, nil)
		underTest := &UpgradeReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			recorder:          &record.FakeRecorder{Events: make(chan string, 10)},
			reconcilerContext: &ctx,
			instance:          spec,
			logger:            logr.Discard(),
		}

		_, err := underTest.Reconcile()
		Expect(err).To(HaveOccurred())
		Expect(IsTerminal(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("downgrade"))
	})
})

func newUpgradeReconciler(mockClient *k8s.MockK8sClient, cluster *opensearchv1.OpenSearchCluster) *UpgradeReconciler {
	reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, cluster, cluster.Spec.NodePools)
	return &UpgradeReconciler{
		client:            mockClient,
		ctx:               context.Background(),
		recorder:          record.NewFakeRecorder(20),
		reconcilerContext: &reconcilerContext,
		instance:          cluster,
		logger:            zap.New().WithName("upgrade-test"),
	}
}

var _ = Describe("Upgrade Reconciler", func() {
	var (
		mockClient *k8s.MockK8sClient
		cluster    *opensearchv1.OpenSearchCluster
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		cluster = &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				UID:       "dummyuid",
			},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					Version: "2.11.0",
				},
				NodePools: []opensearchv1.NodePool{
					{Component: "data", Roles: []string{"data"}, Replicas: 1},
					{Component: "masters", Roles: []string{"cluster_manager"}, Replicas: 1},
				},
			},
			Status: opensearchv1.ClusterStatus{
				Version:     "2.11.0",
				Phase:       opensearchv1.PhaseRunning,
				Initialized: true,
			},
		}
	})

	Describe("stale Upgrader status cleanup", func() {
		It("should clear Upgrader entries when versions are in sync", func() {
			cluster.Status.Phase = opensearchv1.PhaseUpgrading
			cluster.Status.ComponentsStatus = []opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: "data", Status: "Upgraded"},
				{Component: "Upgrader", Description: "masters", Status: "Upgrading"},
				{Component: "RollingRestart", Status: "Finished"},
			}

			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
					updateFn(cluster)
				}).Return(nil).Once()

			underTest := newUpgradeReconciler(mockClient, cluster)
			result, err := underTest.Reconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(cluster.Status.Phase).To(Equal(opensearchv1.PhaseRunning))
			Expect(cluster.Status.ComponentsStatus).To(ConsistOf(
				opensearchv1.ComponentStatus{Component: "RollingRestart", Status: "Finished"},
			))
		})

		It("should reset pool progress when the upgrade target changes", func() {
			cluster.Spec.General.Version = "2.13.0"
			cluster.Status.Version = "2.11.0"
			cluster.Status.Phase = opensearchv1.PhaseUpgrading
			cluster.Status.ComponentsStatus = []opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: "__upgrade_target__", Status: "2.12.0"},
				{Component: "Upgrader", Description: "data", Status: "Upgraded"},
				{Component: "Upgrader", Description: "masters", Status: "Upgrading"},
			}

			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
					updateFn(cluster)
				}).Return(nil).Once()

			underTest := newUpgradeReconciler(mockClient, cluster)
			err := underTest.ensureUpgradeTarget()
			Expect(err).NotTo(HaveOccurred())

			Expect(cluster.Status.ComponentsStatus).To(ConsistOf(
				opensearchv1.ComponentStatus{
					Component:   "Upgrader",
					Description: "__upgrade_target__",
					Status:      "2.13.0",
				},
			))

			pool, status := underTest.findNextNodePoolForUpgrade()
			Expect(status.Status).To(Equal(upgradeStatusPending))
			Expect(pool.Component).To(Equal("data"))
		})

		It("should remove Upgrader entries for pools no longer in the spec", func() {
			cluster.Spec.General.Version = "2.12.0"
			cluster.Status.Version = "2.11.0"
			cluster.Status.ComponentsStatus = []opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: "__upgrade_target__", Status: "2.12.0"},
				{Component: "Upgrader", Description: "data", Status: "Upgraded"},
				{Component: "Upgrader", Description: "gone", Status: "Upgrading"},
			}

			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
					updateFn(cluster)
				}).Return(nil).Once()

			underTest := newUpgradeReconciler(mockClient, cluster)
			err := underTest.cleanOrphanedUpgraderStatuses()
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Status.ComponentsStatus).To(ConsistOf(
				opensearchv1.ComponentStatus{Component: "Upgrader", Description: "__upgrade_target__", Status: "2.12.0"},
				opensearchv1.ComponentStatus{Component: "Upgrader", Description: "data", Status: "Upgraded"},
			))
			Expect(helpers.IsUpgradeInProgress(cluster.Status)).To(BeTrue())
		})
	})

	Describe("custom image pinned", func() {
		It("should sync status.version without running a silent upgrade", func() {
			image := "example.com/opensearch:custom"
			cluster.Spec.General.Version = "2.12.0"
			cluster.Spec.General.ImageSpec = &opensearchv1.ImageSpec{Image: &image}
			cluster.Status.Version = "2.11.0"
			cluster.Status.ComponentsStatus = []opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: "data", Status: "Upgraded"},
			}

			mockClient.On("UpdateOpenSearchClusterStatus", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					updateFn := args.Get(1).(func(*opensearchv1.OpenSearchCluster))
					updateFn(cluster)
				}).Return(nil).Once()

			underTest := newUpgradeReconciler(mockClient, cluster)
			result, err := underTest.Reconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(cluster.Status.Version).To(Equal("2.12.0"))
			Expect(cluster.Status.Phase).To(Equal(opensearchv1.PhaseRunning))
			Expect(cluster.Status.ComponentsStatus).To(BeEmpty())
		})
	})

	Describe("findNextNodePoolForUpgrade", func() {
		It("should skip the upgrade target marker when selecting pools", func() {
			cluster.Status.ComponentsStatus = []opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: "__upgrade_target__", Status: "2.12.0"},
			}
			underTest := newUpgradeReconciler(mockClient, cluster)
			pool, status := underTest.findNextNodePoolForUpgrade()
			Expect(status.Status).To(Equal(upgradeStatusPending))
			Expect(pool.Component).To(Equal("data"))
		})
	})

	Describe("validateUpgrade events", func() {
		It("should reject downgrades", func() {
			cluster.Spec.General.Version = "2.10.0"
			cluster.Status.Version = "2.11.0"
			underTest := newUpgradeReconciler(mockClient, cluster)
			err := underTest.validateUpgrade()
			Expect(err).To(MatchError(ErrVersionDowngrade))
		})
	})
})
