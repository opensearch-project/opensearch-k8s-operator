package controllers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Regression test for #1486: the bootstrap pod must be kept (Status.Initialized stays false)
// until every cluster-manager pod has actually joined the cluster according to _cat/nodes,
// not merely reported Ready.
var _ = Describe("Bootstrap pod removal", Ordered, func() {
	const (
		clusterName = "bootstrap-join-test"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		cluster          = ComposeOpensearchCrd(clusterName, namespace)
		bootstrapName    = builders.BootstrapPodName(&cluster)
		catNodesRoute    = "=~^" + regexp.QuoteMeta(helpers.ClusterURL(&cluster)+"/_cat/nodes")
		registerCatNodes = func(names ...string) {
			body := "["
			for i, n := range names {
				if i > 0 {
					body += ","
				}
				body += fmt.Sprintf(`{"name":%q}`, n)
			}
			osTransport.RegisterResponder(http.MethodGet, catNodesRoute, httpmock.NewStringResponder(200, body+"]"))
		}
		getCluster = func() opensearchv1.OpenSearchCluster {
			c := opensearchv1.OpenSearchCluster{}
			Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&cluster), &c)).To(Succeed())
			return c
		}
		bootstrapPodExists = func() bool {
			return IsCreated(context.Background(), k8sClient, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: bootstrapName, Namespace: namespace}})
		}
	)
	// Keep the cluster small: no dashboards, monitoring or extra volumes, just a 3-replica master pool.
	cluster.Spec.General.AdditionalVolumes = nil
	cluster.Spec.General.Monitoring = opensearchv1.MonitoringConfig{}
	cluster.Spec.Dashboards = opensearchv1.DashboardsConfig{}
	masterPool := cluster.Spec.NodePools[0]
	expectedMasters := builders.ExpectedMasterNodeNames(&cluster)

	BeforeAll(func() {
		clusterURL := helpers.ClusterURL(&cluster)
		for _, u := range []string{clusterURL, clusterURL + "/"} {
			osTransport.RegisterResponder(http.MethodHead, u, httpmock.NewStringResponder(200, "OK"))
			osTransport.RegisterResponder(http.MethodGet, u, httpmock.NewStringResponder(200, `{"name":"test","cluster_name":"test","version":{"number":"2.0.0"}}`))
		}
		registerCatNodes(bootstrapName, expectedMasters[0])

		Expect(CreateNamespace(k8sClient, &cluster)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), &cluster)).Should(Succeed())
		Eventually(func() bool {
			return IsCreated(context.Background(), k8sClient, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: builders.StsName(&cluster, &masterPool), Namespace: namespace}})
		}, timeout, interval).Should(BeTrue())
		Eventually(bootstrapPodExists, timeout, interval).Should(BeTrue())

		// envtest runs no StatefulSet controller or kubelet: create the master pods and mark them Ready ourselves.
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&cluster), &cluster)).To(Succeed())
		for _, name := range expectedMasters {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    map[string]string{helpers.ClusterLabel: clusterName, helpers.NodePoolLabel: masterPool.Component},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "opensearch", Image: "opensearchproject/opensearch:2.0.0"}}},
			}
			// Owned by the cluster so pod events trigger a reconcile.
			Expect(ctrl.SetControllerReference(&cluster, pod, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Create(context.Background(), pod)).To(Succeed())
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			Expect(k8sClient.Status().Update(context.Background(), pod)).To(Succeed())
		}
	})

	It("should keep the bootstrap pod while only some masters have joined the cluster", func() {
		// Wait until the controller has consulted _cat/nodes at least once with all master pods Ready.
		Eventually(func() int {
			return osTransport.GetCallCountInfo()["GET "+catNodesRoute]
		}, timeout, interval).Should(BeNumerically(">", 0))
		Consistently(func() bool { return getCluster().Status.Initialized }, 5*time.Second, interval).Should(BeFalse())
		Expect(bootstrapPodExists()).To(BeTrue())
	})

	It("should mark the cluster initialized and remove the bootstrap pod once all masters have joined", func() {
		registerCatNodes(append([]string{bootstrapName}, expectedMasters...)...)
		// Nudge a reconcile instead of waiting for the periodic requeue.
		Expect(k8sClient.Patch(context.Background(), &cluster, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"test/poke":"1"}}}`)))).To(Succeed())

		Eventually(func() bool { return getCluster().Status.Initialized }, timeout, interval).Should(BeTrue())
		Eventually(bootstrapPodExists, timeout, interval).Should(BeFalse())
	})
})
