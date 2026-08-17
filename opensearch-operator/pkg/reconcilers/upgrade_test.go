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
