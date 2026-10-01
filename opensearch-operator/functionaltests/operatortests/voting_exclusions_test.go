package operatortests

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// GetVotingConfigExclusions reads the cluster state with an index expression
// that matches nothing, so the master does not copy every index's metadata.
// This checks the real cluster still answers with the exclusion names.
var _ = Describe("VotingConfigExclusionsRead", func() {
	var (
		clusterName = "test-cluster"
		namespace   = "default"
		dataManager *TestDataManager
	)

	BeforeEach(func() {
		dataManager, _ = setupDataIntegrityTest(clusterName, namespace)
	})

	It("should return the excluded node names with indices present", func() {
		ctx := context.Background()

		By("Creating an index so the cluster state has index metadata to skip")
		_, err := dataManager.ImportTestData(getDefaultTestData()[:1])
		Expect(err).NotTo(HaveOccurred())

		By("Reading an empty exclusion list")
		Expect(dataManager.osClient.ClearVotingConfigExclusions(ctx, false)).To(Succeed())
		names, err := dataManager.osClient.GetVotingConfigExclusions(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(BeEmpty())

		By("Excluding a master and reading the list back")
		node := clusterName + "-masters-2"
		DeferCleanup(func() {
			Expect(dataManager.osClient.ClearVotingConfigExclusions(ctx, false)).To(Succeed())
		})
		Expect(dataManager.osClient.AddVotingConfigExclusion(ctx, node)).To(Succeed())
		names, err = dataManager.osClient.GetVotingConfigExclusions(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(ConsistOf(node))

		By("Reading an empty list again after the clear")
		Expect(dataManager.osClient.ClearVotingConfigExclusions(ctx, false)).To(Succeed())
		names, err = dataManager.osClient.GetVotingConfigExclusions(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(BeEmpty())
	})
})
