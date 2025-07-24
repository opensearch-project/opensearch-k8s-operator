package helpers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MergeConfigs mutation behavior", func() {
	It("should merge the maps such that right is higher priority than left, and not mutate either argument when merging", func() {
		generalConfig := map[string]string{"http.compression": "true"}
		poolConfig := map[string]string{"node.data": "false"}

		// Save a copy of the original
		original := map[string]string{"http.compression": "true"}

		// Merge and check result
		merged := MergeConfigs(generalConfig, poolConfig)
		expected := map[string]string{"http.compression": "true", "node.data": "false"}
		Expect(merged).To(Equal(expected))

		// Check that longLived was not mutated
		Expect(generalConfig).To(Equal(original))

		// Merge again with a new config
		poolConfig2 := map[string]string{"node.master": "false", "http.compression": "false"}
		expected2 := map[string]string{"http.compression": "false", "node.master": "false"}
		merged2 := MergeConfigs(generalConfig, poolConfig2)
		Expect(merged2).To(Equal(expected2))

		// Still not mutated
		Expect(generalConfig).To(Equal(original))
	})
})
