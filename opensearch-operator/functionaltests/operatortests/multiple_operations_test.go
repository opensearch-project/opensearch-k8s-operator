package operatortests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DataIntegrityMultipleOperations", func() {
	var (
		clusterName = "test-cluster"
		namespace   = "default"
		dataManager *TestDataManager
		operations  *ClusterOperations
		testData    map[string]map[string]interface{}
	)

	BeforeEach(func() {
		dataManager, operations = setupDataIntegrityTest(clusterName, namespace)
	})

	It("should maintain data integrity through multiple operations", func() {
		By("Importing test data")
		var err error
		testData, err = dataManager.ImportTestData(getDefaultTestData())
		Expect(err).NotTo(HaveOccurred())

		By("Operation 1: Scaling up data node pool")
		err = operations.ScaleNodePool(clusterName, "data", 3)
		Expect(err).NotTo(HaveOccurred())
		err = operations.WaitForNodePoolReady(clusterName, "data", 3, time.Minute*10)
		Expect(err).NotTo(HaveOccurred())
		err = dataManager.Reconnect(false)
		Expect(err).NotTo(HaveOccurred())
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())

		By("Operation 2: Scaling down data node pool")
		err = operations.ScaleNodePool(clusterName, "data", 2)
		Expect(err).NotTo(HaveOccurred())
		err = operations.WaitForNodePoolReady(clusterName, "data", 2, time.Minute*10)
		Expect(err).NotTo(HaveOccurred())
		err = dataManager.Reconnect(false)
		Expect(err).NotTo(HaveOccurred())
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())

		By("Operation 3: Checking version")
		version, err := operations.GetClusterVersion(clusterName)
		Expect(err).NotTo(HaveOccurred())
		if version == "2.19.4" {
			By("Upgrading cluster")
			err = operations.UpgradeCluster(clusterName, "3.4.0", "3.4.0")
			Expect(err).NotTo(HaveOccurred())
			err = operations.WaitForUpgradeComplete(clusterName, "docker.io/opensearchproject/opensearch:3.4.0", time.Minute*15)
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.Reconnect(false)
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Final data integrity verification")
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())
		err = dataManager.ValidateClusterHealth(true)
		Expect(err).NotTo(HaveOccurred())
	})
})
