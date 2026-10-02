package operatortests

import (
	"context"
	"strings"
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

	Context("Scale down masters", func() {
		const removedMaster = "test-cluster-masters-2"

		masterNames := func() ([]string, error) {
			nodes, err := dataManager.osClient.CatNodes()
			if err != nil {
				return nil, err
			}
			var names []string
			for _, n := range nodes {
				if strings.HasPrefix(n.Name, clusterName+"-masters-") {
					names = append(names, n.Name)
				}
			}
			return names, nil
		}

		It("should remove a master from the quorum and keep the cluster healthy", func() {
			// The cluster is shared with the other specs: put the pool back to 3
			// even when a step below fails.
			DeferCleanup(func() {
				Eventually(func() error {
					return operations.ScaleNodePool(clusterName, "masters", 3)
				}, time.Minute, time.Second*5).Should(Succeed())
				Expect(operations.WaitForNodePoolReady(clusterName, "masters", 3, time.Minute*10)).To(Succeed())
			})

			By("Importing test data into OpenSearch indices")
			var err error
			testData, err = dataManager.ImportTestData(getDefaultTestData())
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data integrity before scaling")
			Expect(dataManager.ValidateDataIntegrity(testData)).To(Succeed())

			By("Scaling down master node pool: 3 -> 2 replicas")
			Expect(operations.ScaleNodePool(clusterName, "masters", 2)).To(Succeed())

			By("Waiting for scaling to complete (2/2 replicas ready)")
			Expect(operations.WaitForNodePoolReady(clusterName, "masters", 2, time.Minute*10)).To(Succeed())

			By("Reconnecting to cluster")
			Expect(dataManager.Reconnect(false)).To(Succeed())

			By("Verifying the removed master left the cluster")
			Eventually(func() ([]string, error) {
				return masterNames()
			}, time.Minute*5, time.Second*5).Should(And(HaveLen(2), Not(ContainElement(removedMaster))))

			// The operator clears the exclusion on a later reconcile, so it is
			// not necessarily empty as soon as the StatefulSet shows 2/2.
			By("Waiting for the voting config exclusions to be cleared")
			Eventually(func() ([]string, error) {
				return dataManager.osClient.GetVotingConfigExclusions(context.Background())
			}, time.Minute*5, time.Second*5).Should(BeEmpty())

			By("Verifying cluster health is green")
			Eventually(func() error {
				return dataManager.ValidateClusterHealth(false)
			}, time.Minute*5, time.Second*5).Should(Succeed())

			By("Verifying data integrity after scaling")
			Expect(dataManager.ValidateDataIntegrity(testData)).To(Succeed())

			By("Scaling back up to 3 replicas")
			Expect(operations.ScaleNodePool(clusterName, "masters", 3)).To(Succeed())
			Expect(operations.WaitForNodePoolReady(clusterName, "masters", 3, time.Minute*10)).To(Succeed())

			By("Verifying the new master joined and no exclusion is left")
			Eventually(func() ([]string, error) {
				return masterNames()
			}, time.Minute*5, time.Second*5).Should(And(HaveLen(3), ContainElement(removedMaster)))
			Expect(dataManager.osClient.GetVotingConfigExclusions(context.Background())).To(BeEmpty())
		})
	})
})
