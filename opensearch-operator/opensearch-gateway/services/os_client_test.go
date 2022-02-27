package services

/*
import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"time"
)

var _ = Describe("OpensearchCLuster API", func() {
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

	/// ------- Tests logic Check phase -------

	Context("Test opensrearch api are as expected", func() {
		It("Cat Nodes", func() {
			response, err := ClusterClient.CatNodes()
			Expect(err).Should(BeNil())
			Expect(response).ShouldNot(BeEmpty())
			Expect(response[0].Ip).ShouldNot(BeEmpty())
		})
		It("Test Nodes Stats", func() {
			response, err := ClusterClient.NodesStats()
			Expect(err).Should(BeNil())
			Expect(response).ShouldNot(BeNil())
			Expect(response.Nodes).ShouldNot(BeEmpty())
		})
		It("Test Cat Indices", func() {
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1
													}
												  }
											 }`)
			indexName := "cat-indices-test"
			_, err := DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
			_, err = CreateIndex(ClusterClient, indexName, mapping)
			Expect(err).Should(BeNil())
			response, err := ClusterClient.CatIndices()
			Expect(err).Should(BeNil())
			Expect(response).ShouldNot(BeEmpty())
			indexExists := false
			for _, res := range response {
				if indexName == res.Index {
					indexExists = true
					break
				}
			}
			Expect(indexExists).Should(BeTrue())
			_, err = DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())
		})
		It("Test Cat Shards", func() {
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1,
													"number_of_replicas": 1
													}
												  }
											 }`)
			indexName := "cat-shards-test"
			_, err := CreateIndex(ClusterClient, indexName, mapping)
			Expect(err).Should(BeNil())
			var headers = make([]string, 0)
			response, err := ClusterClient.CatShards(headers)
			Expect(err).Should(BeNil())
			Expect(response).ShouldNot(BeEmpty())
			indexExists := false
			for _, res := range response {
				if indexName == res.Index {
					indexExists = true
					break
				}
			}
			Expect(indexExists).Should(BeTrue())
			_, err = DeleteIndex(ClusterClient, indexName)
			Expect(err).Should(BeNil())

		})
		It("Test Put Cluster Settings", func() {
			settingsJson := `{
 					 "transient" : {
    					"indices.recovery.max_bytes_per_sec" : "20mb"
  						}
					}`

			response, err := ClusterClient.PutClusterSettings(settingsJson)
			Expect(err).Should(BeNil())
			Expect(response.Transient).ShouldNot(BeEmpty())

			response, err = ClusterClient.GetClusterSettings()
			Expect(err).Should(BeNil())
			Expect(response.Transient).ShouldNot(BeEmpty())

			settingsJson = `{
 					 "transient" : {
    					"indices.recovery.max_bytes_per_sec" : null
  						}
					}`
			response, err = ClusterClient.PutClusterSettings(settingsJson)
			Expect(err).Should(BeNil())
			indicesSettings := response.Transient["indices"]
			if indicesSettings == nil {
				Expect(true).Should(BeTrue())
			} else {
				maxBytesPerSec := indicesSettings.(map[string]map[string]interface{})
				Expect(maxBytesPerSec["recovery"]["max_bytes_per_sec"]).Should(BeNil())
			}
		})
	})
})
*/
