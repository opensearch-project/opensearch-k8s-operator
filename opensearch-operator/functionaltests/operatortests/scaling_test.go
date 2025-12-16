package operatortests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DataIntegrityScaling", func() {
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

	Context("Scale up", func() {
		It("should maintain data integrity when scaling up node pool", func() {
			By("Importing test data into OpenSearch indices")
			var err error
			testData, err = dataManager.ImportTestData(getDefaultTestData())
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Test data imported: %d indices\n", len(getDefaultTestData()))

			By("Verifying data integrity before scaling")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Data integrity verified before scaling\n")

			By("Scaling up data node pool: 2 -> 4 replicas")
			err = operations.ScaleNodePool(clusterName, "data", 4)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale request submitted: data node pool 2 -> 4 replicas\n")

			By("Waiting for scaling to complete (4/4 replicas ready)")
			err = operations.WaitForNodePoolReady(clusterName, "data", 4, time.Minute*15)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scaling completed: 4/4 replicas ready\n")

			By("Reconnecting to cluster")
			err = dataManager.Reconnect()
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data integrity after scaling")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Data integrity verified after scaling\n")

			By("Verifying cluster health")
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Cluster health verified\n")

			By("Scaling back down to 2 replicas")
			err = operations.ScaleNodePool(clusterName, "data", 2)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale down request submitted: 4 -> 2 replicas\n")

			By("Waiting for scale down to complete")
			err = operations.WaitForNodePoolReady(clusterName, "data", 2, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale down completed: 2/2 replicas ready\n")
		})
	})

	Context("Scale down", func() {
		It("should maintain data integrity when scaling down node pool", func() {
			By("Importing test data into OpenSearch indices")
			var err error
			testData, err = dataManager.ImportTestData(getDefaultTestData())
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Test data imported: %d indices\n", len(getDefaultTestData()))

			By("Verifying data integrity before scaling")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Data integrity verified before scaling\n")

			By("Scaling down data node pool: 2 -> 1 replica")
			err = operations.ScaleNodePool(clusterName, "data", 1)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale down request submitted: 2 -> 1 replica\n")

			By("Waiting for scaling to complete (1/1 replica ready)")
			err = operations.WaitForNodePoolReady(clusterName, "data", 1, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scaling completed: 1/1 replica ready\n")

			By("Reconnecting to cluster")
			err = dataManager.Reconnect()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Reconnected to cluster\n")

			By("Verifying data integrity after scaling")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Data integrity verified after scaling\n")

			By("Verifying cluster health (allowing yellow status)")
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Cluster health verified\n")

			By("Scaling back up to 2 replicas")
			err = operations.ScaleNodePool(clusterName, "data", 2)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale up request submitted: 1 -> 2 replicas\n")

			By("Waiting for scale up to complete")
			err = operations.WaitForNodePoolReady(clusterName, "data", 2, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale up completed: 2/2 replicas ready\n")
		})
	})
})

