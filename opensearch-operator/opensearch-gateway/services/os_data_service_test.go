package services

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"opensearch.opster.io/pkg/builders"
	"strings"
	"time"
)

var _ = Describe("OpensearchCLuster data service tests", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	var (
		ClusterClient *OsClusterClient = nil
	)

	/// ------- Creation Check phase -------

	BeforeEach(func() {
		By("Creating open search client ")
		Eventually(func() bool {
			var err error = nil
			ClusterClient, err = NewOsClusterClient(builders.ClusterUrl(OpensearchCluster), "admin", "admin")
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())
	})
	Context("Data Service Tests logic", func() {
		It("Test Has No Indices With No Replica", func() {
			mapping := strings.NewReader(`{
											 'settings': {
											   'index': {
													'number_of_shards': 1,
													'number_of_replicas': 1
													}
												  }
											 }`)
			indexName := "cat-indices-test"
			CreateIndex(ClusterClient, indexName, mapping)
			hasNoReplicas, err := HasIndicesWithNoReplica(ClusterClient)
			Expect(err).Should(BeNil())
			Expect(hasNoReplicas).ShouldNot(BeTrue())
			DeleteIndex(ClusterClient, indexName)
		})
		It("Test Has Indices With No Replica", func() {
			mapping := strings.NewReader(`{
											 'settings': {
											   'index': {
													'number_of_shards': 1,
													'number_of_replicas': 0
													}
												  }
											 }`)
			indexName := "cat-indices-test"
			CreateIndex(ClusterClient, indexName, mapping)
			hasNoReplicas, err := HasIndicesWithNoReplica(ClusterClient)
			Expect(err).Should(BeNil())
			Expect(hasNoReplicas).Should(BeTrue())
			DeleteIndex(ClusterClient, indexName)
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
})
