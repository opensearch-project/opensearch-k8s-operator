package builders

import (
	"context"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/patch"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Dashboards deployment against the API server", func() {
	It("is in sync right after creation and keeps the pod template clean", func() {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dashboards-sync"}}
		Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())

		cr := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "foobar", Namespace: ns.Name},
			Spec: opensearchv1.ClusterSpec{
				General:    opensearchv1.GeneralConfig{Version: "2.11.0"},
				Dashboards: opensearchv1.DashboardsConfig{Enable: true, Version: "2.11.0", Replicas: 1},
			},
		}
		build := func() *appsv1.Deployment {
			return NewDashboardsDeploymentForCR(cr, nil, nil, map[string]string{"checksum": "abc"})
		}

		desired := build()
		Expect(patch.DefaultAnnotator.SetLastAppliedAnnotation(desired)).To(Succeed())
		Expect(desired.Spec.Template.Annotations).NotTo(HaveKey(patch.LastAppliedConfig))
		Expect(k8sClient.Create(context.Background(), desired)).To(Succeed())

		current := &appsv1.Deployment{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(desired), current)).To(Succeed())
		Expect(current.Spec.Template.Annotations).NotTo(HaveKey(patch.LastAppliedConfig))

		result, err := patch.DefaultPatchMaker.Calculate(current, build(), patch.IgnoreStatusFields())
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsEmpty()).To(BeTrue(), "unexpected patch: %s", string(result.Patch))
	})
})
