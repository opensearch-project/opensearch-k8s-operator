package operatortests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DataIntegrityUpgrade", func() {
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

	It("should maintain data integrity during version upgrade", func() {
		By("Importing test data into OpenSearch indices")
		var err error
		testData, err = dataManager.ImportTestData(getDefaultTestData())
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Test data imported: %d indices\n", len(getDefaultTestData()))

		By("Verifying data integrity before upgrade")
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Data integrity verified before upgrade\n")

		By("Upgrading cluster: OpenSearch 2.19.4 -> 3.4.0, Dashboards 2.19.4 -> 3.4.0")
		err = operations.UpgradeCluster(clusterName, "3.4.0", "3.4.0")
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Upgrade request submitted\n")

		By("Waiting for OpenSearch upgrade to complete (all master pods running new image)")
		err = operations.WaitForUpgradeComplete(clusterName, "docker.io/opensearchproject/opensearch:3.4.0", time.Minute*30)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + OpenSearch upgrade completed successfully\n")

		By("Waiting for Dashboards upgrade to complete")
		err = operations.WaitForDashboardsReady(clusterName, time.Minute*8)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Dashboards upgrade completed successfully\n")

		By("Reconnecting to cluster after upgrade")
		err = dataManager.Reconnect()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Reconnected to cluster\n")

		By("Verifying data integrity after upgrade")
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Data integrity verified after upgrade\n")

		By("Verifying cluster health (allowing yellow status)")
		err = dataManager.ValidateClusterHealth(true) // Allow yellow during upgrade
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Cluster health verified\n")
	})
})
