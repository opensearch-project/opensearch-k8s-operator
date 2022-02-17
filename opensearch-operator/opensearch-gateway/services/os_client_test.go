package services

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"strings"
	"time"
)

var _ = Describe("OpensearchCLuster Controller", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName = "cluster-test-nodes"
		NameSpace   = "default"
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = helpers.ComposeOpensearchCrd(ClusterName, NameSpace)
		/*nodePool          = sts.StatefulSet{}
		cluster2          = opsterv1.OpenSearchCluster{}*/
	)

	/// ------- Creation Check phase -------

	ns := helpers.ComposeNs(ClusterName)
	Context("When create OpenSearch CRD - nodes", func() {
		It("should create cluster NS and CRD instance", func() {
			Expect(helpers.GetK8sClient().Create(context.Background(), &OpensearchCluster)).Should(Succeed())
			By("Create cluster ns ")
			Eventually(func() bool {
				if !helpers.IsNsCreated(helpers.GetK8sClient(), ns) {
					return false
				}
				if !helpers.IsClusterCreated(helpers.GetK8sClient(), OpensearchCluster) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("Test opensrearch api are as expected", func() {
		It("Cat Nodes", func() {
			var clusterClient *OsClusterClient = nil
			var err error = nil
			By("Creating open search client ")
			Eventually(func() error {
				clusterClient, err = builders.NewOsClusterClient(&OpensearchCluster)
				return err

			}, time.Minute*5, 2*time.Second).Should(BeNil())
			response, err := clusterClient.CatNodes()
			Expect(err).Should(BeNil())
			Expect(response).ShouldNot(BeEmpty())
			Expect(response.Ip).ShouldNot(BeEmpty())
		})
		It("Test Nodes Stats", func() {
			var clusterClient *OsClusterClient = nil
			var err error = nil
			By("Creating open search client ")
			Eventually(func() error {
				clusterClient, err = builders.NewOsClusterClient(&OpensearchCluster)
				return err

			}, time.Minute*5, 2*time.Second).Should(BeNil())
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1
													}
												  }
											 }`)
			indexName := "cat-indices-test"
			CreateIndex(clusterClient, indexName, mapping)
			response, err := clusterClient.CatIndices()
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
			DeleteIndex(clusterClient, indexName)
		})
		It("Test Cat Indices", func() {
			var clusterClient *OsClusterClient = nil
			var err error = nil
			By("Creating open search client ")
			Eventually(func() error {
				clusterClient, err = builders.NewOsClusterClient(&OpensearchCluster)
				return err

			}, time.Minute*5, 2*time.Second).Should(BeNil())
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1
													}
												  }
											 }`)
			indexName := "cat-indices-test"
			CreateIndex(clusterClient, indexName, mapping)
			response, err := clusterClient.CatIndices()
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
			DeleteIndex(clusterClient, indexName)
		})
		It("Test Cat Shards", func() {
			var clusterClient *OsClusterClient = nil
			var err error = nil
			By("Creating open search client ")
			Eventually(func() error {
				clusterClient, err = builders.NewOsClusterClient(&OpensearchCluster)
				return err

			}, time.Minute*5, 2*time.Second).Should(BeNil())
			mapping := strings.NewReader(`{
											 "settings": {
											   "index": {
													"number_of_shards": 1,
													"number_of_replicas": 1
													}
												  }
											 }`)
			indexName := "cat-shards-test"
			CreateIndex(clusterClient, indexName, mapping)

			var headers = make([]string, 0)
			response, err := clusterClient.CatShards(headers)
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
			DeleteIndex(clusterClient, indexName)
		})
		It("Test Put Cluster Settings", func() {
			var clusterClient *OsClusterClient = nil
			var err error = nil
			By("Creating open search client ")
			Eventually(func() error {
				clusterClient, err = builders.NewOsClusterClient(&OpensearchCluster)
				return err

			}, time.Minute*5, 2*time.Second).Should(BeNil())
			settingsJson := `{
 					 "transient" : {
    					"indices.recovery.max_bytes_per_sec" : "20mb"
  						}
					}`

			response, err := clusterClient.PutClusterSettings(settingsJson)
			Expect(err).ShouldNot(BeNil())
			Expect(response.Transient).ShouldNot(BeEmpty())

			response, err = clusterClient.GetClusterSettings()
			Expect(err).ShouldNot(BeNil())
			Expect(response.Transient).ShouldNot(BeEmpty())

			settingsJson = `{
 					 "transient" : {
    					"indices.recovery.max_bytes_per_sec" : null
  						}
					}`
			response, err = clusterClient.PutClusterSettings(settingsJson)
			Expect(err).ShouldNot(BeNil())
			indicesSettings := response.Transient["indices"]
			if indicesSettings == nil {
				Expect(true).Should(BeTrue())
			} else {
				maxBytesPerSec := indicesSettings.(map[string]map[string]interface{})
				Expect(maxBytesPerSec["recovery"]["max_bytes_per_sec"]).Should(BeNil())
			}
		})
	})

	/// ------- Deletion Check phase -------

	Context("When deleting OpenSearch CRD ", func() {
		It("should delete cluster NS and resources", func() {

			Expect(helpers.GetK8sClient().Delete(context.Background(), &OpensearchCluster)).Should(Succeed())

			By("Delete cluster ns ")
			Eventually(func() bool {
				return helpers.IsNsDeleted(helpers.GetK8sClient(), ns)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
