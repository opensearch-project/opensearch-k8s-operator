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

			By("Scaling up data node pool: 3 -> 4 replicas")
			err = operations.ScaleNodePool(clusterName, "data", 4)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale request submitted: data node pool 3 -> 4 replicas\n")

			By("Waiting for scaling to complete (4/4 replicas ready)")
			err = operations.WaitForNodePoolReady(clusterName, "data", 4, time.Minute*15)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scaling completed: 4/4 replicas ready\n")

			By("Reconnecting to cluster")
			err = dataManager.Reconnect(false)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data integrity after scaling")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Data integrity verified after scaling\n")

			By("Verifying cluster health")
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Cluster health verified\n")

			By("Scaling back down to 3 replicas")
			err = operations.ScaleNodePool(clusterName, "data", 3)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale down request submitted: 4 -> 3 replicas\n")

			By("Waiting for scale down to complete")
			err = operations.WaitForNodePoolReady(clusterName, "data", 3, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale down completed: 3/3 replicas ready\n")
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

			By("Scaling down data node pool: 3 -> 2 replica")
			err = operations.ScaleNodePool(clusterName, "data", 2)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale down request submitted: 3 -> 2 replica\n")

			By("Waiting for scaling to complete (2/2 replica ready)")
			err = operations.WaitForNodePoolReady(clusterName, "data", 2, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scaling completed: 2/2 replica ready\n")

			By("Reconnecting to cluster")
			err = dataManager.Reconnect(false)
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

			By("Scaling back up to 3 replicas")
			err = operations.ScaleNodePool(clusterName, "data", 3)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale up request submitted: 2 -> 3 replicas\n")

			By("Waiting for scale up to complete")
			err = operations.WaitForNodePoolReady(clusterName, "data", 3, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  + Scale up completed: 3/3 replicas ready\n")
		})
	})
})
