package services

/*
import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"time"
)

var _ = Describe("OpensearchCLuster data service tests", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 120
		interval = time.Second * 1
	)

	var (
		ClusterClient *OsClusterClient = nil
	)

	/// ------- Creation Check phase -------

	BeforeEach(func() {
		By("Creating open search client ")
		Eventually(func() bool {
			clusterClient, err := NewOsClusterClient(TestClusterUrl, TestClusterUserName, TestClusterPassword)
			if err != nil {
				return false
			}
			ClusterClient = clusterClient
			return true
		}, timeout, interval).Should(BeTrue())
	})
	Context("Data Service Tests logic", func() {
		It("Test Has No Indices With No Replica", func() {
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1,
													"number_of_replicas": 1
													}
												  }
											 }`)
			indexName := "indices-no-rep-test"
			_, err := DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
			success, err := CreateIndex(ClusterClient, indexName, mapping)
			Expect(err).Should(BeNil())
			Expect(success == 200 || success == 201).Should(BeTrue())
			hasNoReplicas, err := HasIndicesWithNoReplica(ClusterClient)
			Expect(err).Should(BeNil())
			Expect(hasNoReplicas).ShouldNot(BeTrue())
			_, err = DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
		})
		It("Test Has Indices With No Replica", func() {
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1,
													"number_of_replicas": 0
													}
												  }
											 }`)
			indexName := "indices-with-rep-test"
			_, err := DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
			hasNoReplicas := false
			success, err := CreateIndex(ClusterClient, indexName, mapping)
			Expect(err).Should(BeNil())
			Expect(success == 200 || success == 201).Should(BeTrue())
			hasNoReplicas, err = HasIndicesWithNoReplica(ClusterClient)
			Expect(err).Should(BeNil())
			Expect(hasNoReplicas).Should(BeTrue())
			_, err = DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
		})
		It("Test Node Exclude", func() {
			nodeExcluded, err := AppendExcludeNodeHost(ClusterClient, "not-exists-node")
			Expect(err).Should(BeNil())
			Expect(nodeExcluded).Should(BeTrue())
			nodeExcluded, err = RemoveExcludeNodeHost(ClusterClient, "not-exists-node")
			Expect(err).Should(BeNil())
			Expect(nodeExcluded).Should(BeTrue())
		})
	})
})*/
