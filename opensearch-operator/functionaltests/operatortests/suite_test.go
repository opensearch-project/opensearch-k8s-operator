package operatortests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Global setup/teardown for data integrity scenarios.
// This ensures the test-cluster cluster exists before any
// focused scenario (e.g. "Scale up scenario") is executed.

var _ = BeforeSuite(func() {
	By("Creating OpenSearchCluster 'test-cluster'")
	// Create the OpenSearchCluster used by all data integrity tests.
	// The manifest lives under test-cluster.yaml.
	err := CreateKubernetesObjects("test-cluster")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if !ShouldSkipCleanup() {
		By("Cleaning up NodePort service (port 30000)")
		const nodePort int32 = 30000
		_ = CleanUpNodePort("default", nodePort)

		By("Cleaning up OpenSearchCluster and related resources")
		Cleanup("test-cluster")
	} else {
		By("Skipping cleanup (SKIP_CLEANUP is set) - resources left in place for debugging")
	}
})
