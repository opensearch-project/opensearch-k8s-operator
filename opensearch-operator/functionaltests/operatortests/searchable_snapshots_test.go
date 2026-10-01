package operatortests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SearchableSnapshots", Ordered, func() {
	const (
		name       = "searchable-snapshots"
		namespace  = "default"
		repository = "snapshots"
		index      = "test-products"
	)
	var (
		dataManager *TestDataManager
		operations  *ClusterOperations
	)

	// call sends a request to the cluster and returns the status code and body.
	call := func(method, path, body string) (int, string) {
		req, err := http.NewRequest(method, path, strings.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		res, err := dataManager.osClientRaw.Perform(req)
		Expect(err).NotTo(HaveOccurred())
		data, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		Expect(err).NotTo(HaveOccurred())
		return res.StatusCode, string(data)
	}

	hitCount := func() int {
		status, body := call(http.MethodGet, "/"+index+"/_search?size=0&track_total_hits=true", "")
		if status != http.StatusOK {
			return -1
		}
		var res struct {
			Hits struct {
				Total struct {
					Value int `json:"value"`
				} `json:"total"`
			} `json:"hits"`
		}
		Expect(json.Unmarshal([]byte(body), &res)).To(Succeed())
		return res.Hits.Total.Value
	}

	clusterHealth := func() string {
		health, err := dataManager.osClient.GetHealth()
		if err != nil {
			return err.Error()
		}
		return health.Status
	}

	BeforeAll(func() {
		Expect(CreateKubernetesObjects(name)).To(Succeed())
		operations = NewClusterOperations(k8sClient, namespace)

		By("Waiting for the hot and warm node pools")
		Expect(operations.WaitForNodePoolReady(name, "hot", 1, time.Minute*15)).To(Succeed())
		Expect(operations.WaitForNodePoolReady(name, "warm", 1, time.Minute*15)).To(Succeed())

		var err error
		dataManager, err = NewTestDataManager(k8sClient, name, namespace, false)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !ShouldSkipCleanup() {
			_ = CleanUpNodePort(namespace, 30000)
			Cleanup(name)
		}
	})

	It("should serve a restored remote_snapshot index from the warm pool", func() {
		snapshot := fmt.Sprintf("snap-%d", time.Now().Unix())

		By("Waiting for the operator to register the snapshot repository")
		Eventually(func() int {
			status, _ := call(http.MethodGet, "/_snapshot/"+repository, "")
			return status
		}, time.Minute*5, time.Second*5).Should(Equal(http.StatusOK))

		By("Indexing documents and taking a snapshot")
		testIndex := getDefaultTestData()[0]
		testIndex.Settings = `{"settings": {"index": {"number_of_shards": 1, "number_of_replicas": 0}}}`
		_, err := dataManager.ImportTestData([]TestIndex{testIndex})
		Expect(err).NotTo(HaveOccurred())
		status, body := call(http.MethodPut, fmt.Sprintf("/_snapshot/%s/%s?wait_for_completion=true", repository, snapshot), `{"indices": "`+index+`"}`)
		Expect(status).To(Equal(http.StatusOK), body)
		Expect(body).To(ContainSubstring(`"state":"SUCCESS"`))

		By("Restoring the index as a searchable snapshot")
		status, body = call(http.MethodDelete, "/"+index, "")
		Expect(status).To(Equal(http.StatusOK), body)
		status, body = call(http.MethodPost, fmt.Sprintf("/_snapshot/%s/%s/_restore?wait_for_completion=true", repository, snapshot),
			`{"indices": "`+index+`", "storage_type": "remote_snapshot"}`)
		Expect(status).To(Equal(http.StatusOK), body)

		By("Checking the store type, shard placement and search results")
		status, body = call(http.MethodGet, "/"+index+"/_settings/index.store.type", "")
		Expect(status).To(Equal(http.StatusOK), body)
		Expect(body).To(ContainSubstring(`"type":"remote_snapshot"`))
		Eventually(func() string {
			_, shards := call(http.MethodGet, "/_cat/shards/"+index+"?h=state,node", "")
			return strings.TrimSpace(shards)
		}, time.Minute*2, time.Second*5).Should(Equal("STARTED " + name + "-warm-0"))
		Expect(hitCount()).To(Equal(len(testIndex.Documents)))

		By("Rolling-restarting the warm pool")
		pod := corev1.Pod{}
		podKey := types.NamespacedName{Name: name + "-warm-0", Namespace: namespace}
		Expect(k8sClient.Get(context.Background(), podKey, &pod)).To(Succeed())
		oldUID := pod.UID
		cluster := opensearchv1.OpenSearchCluster{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &cluster)).To(Succeed())
		for i := range cluster.Spec.NodePools {
			if cluster.Spec.NodePools[i].Component == "warm" {
				cluster.Spec.NodePools[i].Annotations = map[string]string{"functionaltests/restart": snapshot}
			}
		}
		Expect(k8sClient.Update(context.Background(), &cluster)).To(Succeed())
		Eventually(func() bool {
			if err := k8sClient.Get(context.Background(), podKey, &pod); err != nil || pod.UID == oldUID {
				return false
			}
			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodReady {
					return c.Status == corev1.ConditionTrue
				}
			}
			return false
		}, time.Minute*10, time.Second*5).Should(BeTrue())

		By("Checking the index is green and searchable again")
		Eventually(clusterHealth, time.Minute*5, time.Second*5).Should(Equal("green"))
		Expect(hitCount()).To(Equal(len(testIndex.Documents)))
	})
})
