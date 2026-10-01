package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("dashboards.replicas: 0 survives operator writes", func() {
	const (
		namespace = "dashboards-replicas-zero"
		timeout   = time.Second * 30
		interval  = time.Millisecond * 250
	)
	ctx := context.Background()

	BeforeEach(func() {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(k8sClient.Create(ctx, ns)).To(Or(Succeed(), MatchError(ContainSubstring("already exists"))))
	})

	It("is kept at 0 after the cluster controller adds its finalizer", func() {
		const name = "finalizer-flip"
		// Create the CR the way kubectl does: replicas: 0 is present in the request
		// body, so the CRD structural default (1) does not apply and 0 is persisted.
		u := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "opensearch.org/v1",
			"kind":       "OpenSearchCluster",
			"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
			"spec": map[string]interface{}{
				"general": map[string]interface{}{"version": "2.19.6", "serviceName": name},
				"dashboards": map[string]interface{}{
					"enable": true, "replicas": int64(0), "version": "2.19.6",
				},
				"nodePools": []interface{}{map[string]interface{}{
					"component": "nodes", "replicas": int64(1), "roles": []interface{}{"cluster_manager", "data"},
				}},
			},
		}}
		Expect(k8sClient.Create(ctx, u)).To(Succeed())
		Expect(u.Object["spec"].(map[string]interface{})["dashboards"].(map[string]interface{})["replicas"]).
			To(BeEquivalentTo(0), "precondition: API server persisted replicas: 0")

		// The running OpenSearchClusterReconciler adds its finalizer with a full
		// typed Update; wait for that write.
		got := &opensearchv1.OpenSearchCluster{}
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, got); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(got, "Opensearch")
		}, timeout, interval).Should(BeTrue(), "controller should add its finalizer")

		Expect(got.Spec.Dashboards.Replicas).To(HaveValue(Equal(int32(0))))
	})

	It("is kept at 0 by a typed Create", func() {
		// What the migration controller does with the converted CR.
		const name = "typed-create-flip"
		created := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: opensearchv1.ClusterSpec{
				General:    opensearchv1.GeneralConfig{Version: "2.19.6", ServiceName: name},
				Dashboards: opensearchv1.DashboardsConfig{Enable: false, Replicas: ptr.To(int32(0)), Version: "2.19.6"},
				NodePools:  []opensearchv1.NodePool{{Component: "nodes", Replicas: 1, Roles: []string{"cluster_manager", "data"}}},
			},
		}
		Expect(k8sClient.Create(ctx, created)).To(Succeed())

		// The Create response already carries the defaulted object.
		Expect(created.Spec.Dashboards.Replicas).To(HaveValue(Equal(int32(0))))
	})
})
