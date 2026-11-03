package util

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

var _ = Describe("status_types", func() {

	Describe("DefaultLimits", func() {
		It("matches every historical max constant", func() {
			l := DefaultLimits()
			Expect(l.Indices).To(Equal(maxIndicesShown))
			Expect(l.Segments).To(Equal(maxSegmentsShown))
			Expect(l.UnassignedShards).To(Equal(maxUnassignedShards))
			Expect(l.Nodes).To(Equal(maxNodesShown))
			Expect(l.AllocationRows).To(Equal(maxAllocationRows))
			Expect(l.RecoveryRows).To(Equal(maxRecoveryRows))
			Expect(l.ThreadPoolRows).To(Equal(maxThreadPoolRows))
			Expect(l.PendingTasks).To(Equal(maxPendingTasks))
			Expect(l.ProblemIndices).To(Equal(maxProblemIndices))
			Expect(l.AllocationExplain).To(Equal(maxAllocationExplain))
			Expect(l.DeciderNodes).To(Equal(maxDeciderNodes))
		})
	})

	Describe("UnlimitedLimits", func() {
		It("returns a zero-value Limits so every field is <= 0 (no cap)", func() {
			l := UnlimitedLimits()
			Expect(l).To(Equal(Limits{}))
		})
	})

	Describe("capCount", func() {
		It("returns n when n < limit", func() {
			Expect(capCount(3, 10)).To(Equal(3))
		})

		It("returns limit when n > limit", func() {
			Expect(capCount(15, 10)).To(Equal(10))
		})

		It("returns n when n == limit (no truncation at exact boundary)", func() {
			Expect(capCount(10, 10)).To(Equal(10))
		})

		It("returns n when limit is 0 (unlimited)", func() {
			Expect(capCount(100, 0)).To(Equal(100))
		})

		It("returns n when limit is negative (unlimited)", func() {
			Expect(capCount(50, -1)).To(Equal(50))
		})
	})

	Describe("NewClusterData", func() {
		It("stores every field passed to the constructor", func() {
			health := responses.ClusterHealthResponse{Status: "green"}
			nodes := []responses.CatNodesResponse{{Name: "n1"}}
			indices := []responses.CatIndicesResponse{{Index: "i1"}}
			shards := []responses.CatShardsResponse{{Index: "i1", Shard: "0"}}
			allocation := []responses.CatAllocationResponse{{Node: "n1"}}
			managers := []responses.CatClusterManagerResponse{{Node: "n1"}}
			segments := []responses.CatSegmentsResponse{{Index: "i1"}}
			pending := []responses.CatPendingTasksResponse{{Source: "put-mapping"}}
			threadPool := []responses.CatThreadPoolResponse{{Name: "write"}}
			recovery := []responses.CatRecoveryResponse{{Index: "i1"}}
			nodesStats := responses.NodesStatsResponse{}
			explains := []responses.AllocationExplainResponse{{Index: "i1"}}

			d := NewClusterData(health, nodes, indices, shards, allocation, managers, segments, pending, threadPool, recovery, nodesStats, explains)

			Expect(d.health).To(Equal(health))
			Expect(d.nodes).To(Equal(nodes))
			Expect(d.indices).To(Equal(indices))
			Expect(d.shards).To(Equal(shards))
			Expect(d.allocation).To(Equal(allocation))
			Expect(d.clusterManager).To(Equal(managers))
			Expect(d.segments).To(Equal(segments))
			Expect(d.pendingTasks).To(Equal(pending))
			Expect(d.threadPool).To(Equal(threadPool))
			Expect(d.recovery).To(Equal(recovery))
			Expect(d.nodesStats).To(Equal(nodesStats))
			Expect(d.allocationExplain).To(Equal(explains))
		})
	})

	Describe("NewClusterDataErr", func() {
		It("stores every error field passed to the constructor", func() {
			errs := [12]error{}
			for i := range errs {
				errs[i] = fmt.Errorf("err-%d", i)
			}
			e := NewClusterDataErr(errs[0], errs[1], errs[2], errs[3], errs[4], errs[5], errs[6], errs[7], errs[8], errs[9], errs[10], errs[11])

			Expect(e.healthErr).To(Equal(errs[0]))
			Expect(e.nodesErr).To(Equal(errs[1]))
			Expect(e.indicesErr).To(Equal(errs[2]))
			Expect(e.shardsErr).To(Equal(errs[3]))
			Expect(e.allocationErr).To(Equal(errs[4]))
			Expect(e.clusterManagerErr).To(Equal(errs[5]))
			Expect(e.segmentsErr).To(Equal(errs[6]))
			Expect(e.pendingTasksErr).To(Equal(errs[7]))
			Expect(e.threadPoolErr).To(Equal(errs[8]))
			Expect(e.recoveryErr).To(Equal(errs[9]))
			Expect(e.nodesStatsErr).To(Equal(errs[10]))
			Expect(e.allocationExplainErr).To(Equal(errs[11]))
		})
	})

	Describe("ClusterDataErr.allFailed", func() {
		It("returns true when every primary source has an error", func() {
			boom := fmt.Errorf("boom")
			e := ClusterDataErr{
				healthErr: boom, nodesErr: boom, indicesErr: boom, shardsErr: boom,
				allocationErr: boom, clusterManagerErr: boom, segmentsErr: boom,
				pendingTasksErr: boom, threadPoolErr: boom, recoveryErr: boom,
				nodesStatsErr: boom,
			}
			Expect(e.allFailed()).To(BeTrue())
		})

		It("returns false when at least one primary source succeeded", func() {
			boom := fmt.Errorf("boom")
			// healthErr is nil — one source succeeded
			e := ClusterDataErr{
				nodesErr: boom, indicesErr: boom, shardsErr: boom,
				allocationErr: boom, clusterManagerErr: boom, segmentsErr: boom,
				pendingTasksErr: boom, threadPoolErr: boom, recoveryErr: boom,
				nodesStatsErr: boom,
			}
			Expect(e.allFailed()).To(BeFalse())
		})

		It("ignores allocationExplainErr (derived source) when checking allFailed", func() {
			boom := fmt.Errorf("boom")
			// Every primary source failed, but allocationExplainErr is nil.
			// allFailed must still return true.
			e := ClusterDataErr{
				healthErr: boom, nodesErr: boom, indicesErr: boom, shardsErr: boom,
				allocationErr: boom, clusterManagerErr: boom, segmentsErr: boom,
				pendingTasksErr: boom, threadPoolErr: boom, recoveryErr: boom,
				nodesStatsErr:        boom,
				allocationExplainErr: nil,
			}
			Expect(e.allFailed()).To(BeTrue())
		})
	})
})
