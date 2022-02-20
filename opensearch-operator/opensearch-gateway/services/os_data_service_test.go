package services

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	"strings"
	"testing"
	"time"
)

func TestOsDataService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})

}

var _ = BeforeSuite(func() {
	helpers.BeforeSuiteLogic()

}, 60)

var _ = AfterSuite(func() {
	helpers.AfterSuiteLogic()
})

var _ = Describe("OpensearchCLuster data service tests", func() {
	//	ctx := context.Background()

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ClusterName = "cluster-test-nodes"
		NameSpace   = "default"
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster                  = helpers.ComposeOpensearchCrd(ClusterName, NameSpace)
		ClusterClient     *OsClusterClient = nil
	)

	/// ------- Creation Check phase -------

	ns := helpers.ComposeNs(ClusterName)
	BeforeEach(func() {
		By("Creating open search client ")
		Eventually(func() bool {
			var err error = nil
			if !helpers.IsNsCreated(helpers.K8sClient, ns) {
				return false
			}
			if !helpers.IsClusterCreated(helpers.K8sClient, OpensearchCluster) {
				return false
			}
			ClusterClient, err = NewOsClusterClient(builders.ClusterUrl(&OpensearchCluster), "admin", "admin")
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
