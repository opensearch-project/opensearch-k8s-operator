package builders

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("initial master nodes", func() {
	cluster := func(initialized bool, pools ...opensearchv1.NodePool) *opensearchv1.OpenSearchCluster {
		return &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
			Spec: opensearchv1.ClusterSpec{
				General:   opensearchv1.GeneralConfig{Version: "2.7.0"},
				NodePools: pools,
			},
			Status: opensearchv1.ClusterStatus{Initialized: initialized},
		}
	}
	masters := opensearchv1.NodePool{Component: "masters", Replicas: 3, Roles: []string{"cluster_manager"}}
	idle := opensearchv1.NodePool{Component: "idle", Replicas: 0, Roles: []string{"cluster_manager"}}
	data := opensearchv1.NodePool{Component: "data", Replicas: 2, Roles: []string{"data"}}

	It("lists pod-0 of each master pool that has replicas and skips the rest", func() {
		cr := cluster(false, masters, idle, data)
		value, include := ResolveInitialMasterNodes(cr, "", false, false)
		Expect(include).To(BeTrue())
		Expect(value).To(Equal("demo-masters-0"))
	})

	It("uses the bootstrap pod name while that pod exists and the cluster is not initialized", func() {
		cr := cluster(false, masters)
		value, include := ResolveInitialMasterNodes(cr, "demo-masters-0", true, true)
		Expect(include).To(BeTrue())
		Expect(value).To(Equal("demo-bootstrap-0"))
	})

	It("keeps the value already on an initialized StatefulSet", func() {
		cr := cluster(true, masters)
		value, include := ResolveInitialMasterNodes(cr, "demo-bootstrap-0", true, false)
		Expect(include).To(BeTrue())
		Expect(value).To(Equal("demo-bootstrap-0"))
	})

	It("omits the setting on a pool created after the cluster is initialized", func() {
		cr := cluster(true, masters)
		value, include := ResolveInitialMasterNodes(cr, "", false, false)
		Expect(include).To(BeFalse())
		Expect(value).To(BeEmpty())
	})

	It("removes the env var when told to leave it off", func() {
		cr := cluster(false, masters)
		sts := NewSTSForNodePool("admin", cr, data, "hash", nil, nil)
		ApplyInitialMasterNodesEnv(sts, "", false)
		_, found := InitialMasterNodesFromEnv(sts.Spec.Template.Spec.Containers[0].Env)
		Expect(found).To(BeFalse())
	})

	It("reads the env var back off a pod template", func() {
		value, found := InitialMasterNodesFromEnv([]corev1.EnvVar{{Name: "cluster.initial_master_nodes", Value: "demo-masters-0"}})
		Expect(found).To(BeTrue())
		Expect(value).To(Equal("demo-masters-0"))
	})
})
