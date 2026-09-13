package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A cluster re-created over retained data PVCs re-forms its old cluster from disk; the
// bootstrap pod would start a second, unrelated cluster that no node ever joins.
var _ = Describe("Bootstrap pod over retained PVCs", Ordered, func() {
	const (
		clusterName = "bootstrap-retained-pvc"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		cluster       = ComposeOpensearchCrd(clusterName, namespace)
		bootstrapName = builders.BootstrapPodName(&cluster)
	)
	cluster.Spec.General.AdditionalVolumes = nil
	cluster.Spec.General.Monitoring = opensearchv1.MonitoringConfig{}
	cluster.Spec.Dashboards = opensearchv1.DashboardsConfig{}
	// Retained data only exists for PVC-backed pools; the composed CR defaults to emptyDir.
	cluster.Spec.NodePools[0].Persistence = nil
	masterPool := cluster.Spec.NodePools[0]

	BeforeAll(func() {
		Expect(CreateNamespace(k8sClient, &cluster)).Should(Succeed())
		// The PVC a previous incarnation's StatefulSet left behind: <claim>-<sts>-<ordinal>.
		retained := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-" + builders.StsName(&cluster, &masterPool) + "-0",
				Namespace: namespace,
				Labels:    map[string]string{helpers.ClusterLabel: clusterName, helpers.NodePoolLabel: masterPool.Component},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			},
		}
		Expect(k8sClient.Create(context.Background(), retained)).To(Succeed())
		Expect(k8sClient.Create(context.Background(), &cluster)).Should(Succeed())
		Eventually(func() bool {
			return IsCreated(context.Background(), k8sClient, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: builders.StsName(&cluster, &masterPool), Namespace: namespace}})
		}, timeout, interval).Should(BeTrue())
	})

	It("should not create a bootstrap pod when a master pool's data PVC already exists", func() {
		Consistently(func() bool {
			return IsCreated(context.Background(), k8sClient, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: bootstrapName, Namespace: namespace}})
		}, 5*time.Second, interval).Should(BeFalse())
	})

	It("should mark the cluster initialized without waiting for the masters to join", func() {
		c := opensearchv1.OpenSearchCluster{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&cluster), &c)).To(Succeed())
		Expect(c.Status.Initialized).To(BeTrue())
	})
})
