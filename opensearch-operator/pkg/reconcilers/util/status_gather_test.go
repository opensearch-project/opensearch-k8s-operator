package util

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

var _ = Describe("status_gather", func() {

	// -------------------------------------------------------------------------
	// GatherAllocationExplainWithLimit
	// -------------------------------------------------------------------------

	Describe("GatherAllocationExplainWithLimit", func() {
		It("returns nil,nil immediately when shardsErr is non-nil", func() {
			explains, err := GatherAllocationExplainWithLimit(nil, nil, fmt.Errorf("shards unavailable"), 5)
			Expect(explains).To(BeNil())
			Expect(err).To(BeNil())
		})

		It("returns nil,nil when there are no unassigned shards", func() {
			shards := []responses.CatShardsResponse{
				{Index: "idx", Shard: "0", PrimaryOrReplica: "p", State: "STARTED"},
				{Index: "idx", Shard: "1", PrimaryOrReplica: "r", State: "STARTED"},
			}
			explains, err := GatherAllocationExplainWithLimit(nil, shards, nil, 5)
			Expect(explains).To(BeNil())
			Expect(err).To(BeNil())
		})

	})

	// -------------------------------------------------------------------------
	// unassignedSorted
	// -------------------------------------------------------------------------

	Describe("unassignedSorted", func() {
		It("keeps only UNASSIGNED shards, ordered by index then numeric shard", func() {
			got := unassignedSorted([]responses.CatShardsResponse{
				{Index: "zzz", Shard: "0", State: "UNASSIGNED"},
				{Index: "aaa", Shard: "10", State: "UNASSIGNED"},
				{Index: "aaa", Shard: "2", State: "UNASSIGNED"},
				{Index: "aaa", Shard: "0", State: "STARTED"}, // assigned: dropped
			})

			// Within "aaa", shard "2" sorts before "10" (numeric, not lexical).
			type key struct{ index, shard string }
			var order []key
			for _, s := range got {
				order = append(order, key{s.Index, s.Shard})
			}
			Expect(order).To(Equal([]key{
				{"aaa", "2"},
				{"aaa", "10"},
				{"zzz", "0"},
			}))
		})

		It("returns nil when no shards are unassigned", func() {
			Expect(unassignedSorted([]responses.CatShardsResponse{
				{Index: "idx", Shard: "0", State: "STARTED"},
			})).To(BeNil())
		})
	})
})
