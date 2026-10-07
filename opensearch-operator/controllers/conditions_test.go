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
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cluster conditions", Ordered, func() {
	const (
		clusterName = "conditions-test"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Millisecond * 250
	)
	var (
		cluster    = ComposeOpensearchCrd(clusterName, namespace)
		getCluster = func(g Gomega) opensearchv1.OpenSearchCluster {
			c := opensearchv1.OpenSearchCluster{}
			g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&cluster), &c)).To(Succeed())
			return c
		}
		updateSpec = func(mutate func(*opensearchv1.OpenSearchCluster)) int64 {
			var generation int64
			Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				c := opensearchv1.OpenSearchCluster{}
				if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&cluster), &c); err != nil {
					return err
				}
				mutate(&c)
				if err := k8sClient.Update(context.Background(), &c); err != nil {
					return err
				}
				generation = c.Generation
				return nil
			})).To(Succeed())
			return generation
		}
	)
	// A single PVC-backed master pool, no dashboards, monitoring or extra volumes.
	cluster.Spec.General.AdditionalVolumes = nil
	cluster.Spec.General.Monitoring = opensearchv1.MonitoringConfig{}
	cluster.Spec.Dashboards = opensearchv1.DashboardsConfig{}
	cluster.Spec.NodePools = cluster.Spec.NodePools[:1]
	cluster.Spec.NodePools[0].Persistence = nil

	BeforeAll(func() {
		masters := builders.ExpectedMasterNodeNames(&cluster)
		body := fmt.Sprintf(`[{"name":%q}`, builders.BootstrapPodName(&cluster))
		for _, name := range masters {
			body += fmt.Sprintf(`,{"name":%q}`, name)
		}
		osTransport.RegisterResponder(http.MethodGet, catNodesRoute, httpmock.NewStringResponder(200, body+"]"))
		osTransport.RegisterResponder(http.MethodGet, "=~^"+regexp.QuoteMeta(helpers.ClusterURL(&cluster))+"/_cluster/health",
			httpmock.NewStringResponder(200, `{"status":"green","number_of_nodes":3}`))

		Expect(CreateNamespace(k8sClient, &cluster)).To(Succeed())
		Expect(k8sClient.Create(context.Background(), &cluster)).To(Succeed())
	})

	AfterAll(func() {
		registerDefaultCatNodesResponder()
	})

	It("reports Ready for the current generation once the StatefulSets converge", func() {
		Eventually(func(g Gomega) {
			g.Expect(MarkStsReady(k8sClient, namespace)).To(Succeed())
			c := getCluster(g)
			g.Expect(c.Status.ObservedGeneration).To(Equal(c.Generation))
			g.Expect(c.Status.Health).To(Equal(opensearchv1.OpenSearchGreenHealth))
			ready := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.ObservedGeneration).To(Equal(c.Generation))
			degraded := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionDegraded)
			g.Expect(degraded).NotTo(BeNil())
			g.Expect(degraded.Status).To(Equal(metav1.ConditionFalse))
		}, timeout, interval).Should(Succeed())
	})

	It("reports Ready=False for a spec change until the StatefulSet rolls it out", func() {
		generation := updateSpec(func(c *opensearchv1.OpenSearchCluster) {
			c.Spec.NodePools[0].Resources.Limits[corev1.ResourceMemory] = resource.MustParse("3Gi")
		})

		Eventually(func(g Gomega) {
			c := getCluster(g)
			g.Expect(c.Status.ObservedGeneration).To(Equal(generation))
			ready := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(opensearchv1.ReasonPodsNotReady))
			g.Expect(ready.ObservedGeneration).To(Equal(generation))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(MarkStsReady(k8sClient, namespace)).To(Succeed())
			ready := apimeta.FindStatusCondition(getCluster(g).Status.Conditions, opensearchv1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.ObservedGeneration).To(Equal(generation))
		}, timeout, interval).Should(Succeed())
	})

	It("reports Degraded while a reconciler keeps failing, without stamping Ready", func() {
		readyGeneration := getCluster(Default).Generation
		// envtest's OpenSearch has no snapshot API, so the snapshot repository reconciler fails every pass.
		generation := updateSpec(func(c *opensearchv1.OpenSearchCluster) {
			c.Spec.General.SnapshotRepositories = []opensearchv1.SnapshotRepoConfig{{Name: "backups", Type: "fs"}}
		})

		Eventually(func(g Gomega) {
			c := getCluster(g)
			g.Expect(c.Status.ObservedGeneration).To(Equal(generation))
			degraded := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionDegraded)
			g.Expect(degraded.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(degraded.Reason).To(Equal(opensearchv1.ReasonReconcileError))
			g.Expect(degraded.Message).To(HavePrefix("snapshot_repository reconciler: "))
			ready := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.ObservedGeneration).To(Equal(readyGeneration))
		}, timeout, interval).Should(Succeed())

		generation = updateSpec(func(c *opensearchv1.OpenSearchCluster) {
			c.Spec.General.SnapshotRepositories = nil
		})
		Eventually(func(g Gomega) {
			c := getCluster(g)
			degraded := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionDegraded)
			g.Expect(degraded.Status).To(Equal(metav1.ConditionFalse))
			ready := apimeta.FindStatusCondition(c.Status.Conditions, opensearchv1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.ObservedGeneration).To(Equal(generation))
		}, timeout, interval).Should(Succeed())
	})
})
