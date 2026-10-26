package operatortests

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DataIntegrityMasterScaling", func() {
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

	Context("Scale down masters", func() {
		removedMaster := clusterName + "-masters-2"

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
				Expect(operations.WaitForNodePoolReady(clusterName, "masters", 3, time.Minute*5)).To(Succeed())
			})

			By("Importing test data into OpenSearch indices")
			var err error
			testData, err = dataManager.ImportTestData(getDefaultTestData())
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data integrity before scaling")
			Expect(dataManager.ValidateDataIntegrity(testData)).To(Succeed())

			By("Verifying the master to remove is a voter")
			removedID, err := dataManager.GetNodeID(removedMaster)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataManager.GetCommittedVotingConfig()).To(ContainElement(removedID))

			By("Scaling down master node pool: 3 -> 2 replicas")
			Expect(operations.ScaleNodePool(clusterName, "masters", 2)).To(Succeed())

			By("Waiting for scaling to complete (2/2 replicas ready)")
			Expect(operations.WaitForNodePoolReady(clusterName, "masters", 2, time.Minute*5)).To(Succeed())

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

			// Auto-shrink keeps a departed voter while fewer than 3 would remain,
			// so a master removed without an exclusion stays in the config.
			By("Verifying the removed master left the voting configuration")
			Eventually(func() ([]string, error) {
				return dataManager.GetCommittedVotingConfig()
			}, time.Minute*2, time.Second*5).ShouldNot(ContainElement(removedID))

			By("Verifying cluster health is green")
			Eventually(func() error {
				return dataManager.ValidateClusterHealth(false)
			}, time.Minute*5, time.Second*5).Should(Succeed())

			By("Verifying data integrity after scaling")
			Expect(dataManager.ValidateDataIntegrity(testData)).To(Succeed())

			By("Scaling back up to 3 replicas")
			Expect(operations.ScaleNodePool(clusterName, "masters", 3)).To(Succeed())
			Expect(operations.WaitForNodePoolReady(clusterName, "masters", 3, time.Minute*5)).To(Succeed())

			By("Verifying the new master joined and no exclusion is left")
			Eventually(func() ([]string, error) {
				return masterNames()
			}, time.Minute*5, time.Second*5).Should(And(HaveLen(3), ContainElement(removedMaster)))
			Expect(dataManager.osClient.GetVotingConfigExclusions(context.Background())).To(BeEmpty())
		})
	})
})
