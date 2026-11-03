package util

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

var _ = Describe("status_render", func() {

	// -------------------------------------------------------------------------
	// parseSizeToBytes
	// -------------------------------------------------------------------------

	Describe("parseSizeToBytes", func() {
		DescribeTable("converts OpenSearch size strings to bytes",
			func(input string, expected int64) {
				Expect(parseSizeToBytes(input)).To(Equal(expected))
			},
			Entry("bytes", "512b", int64(512)),
			Entry("kilobytes", "1kb", int64(1024)),
			Entry("megabytes", "2mb", int64(2*1<<20)),
			Entry("gigabytes", "1.5gb", int64(float64(1.5)*float64(1<<30))),
			Entry("terabytes", "1tb", int64(1<<40)),
			Entry("case-insensitive", "10MB", int64(10*1<<20)),
			Entry("leading/trailing whitespace", "  4kb  ", int64(4*1024)),
			Entry("unrecognised format returns 0", "bad", int64(0)),
			Entry("empty string returns 0", "", int64(0)),
		)
	})

	// -------------------------------------------------------------------------
	// formatBytes
	// -------------------------------------------------------------------------

	Describe("formatBytes", func() {
		DescribeTable("renders byte counts as human-readable binary sizes",
			func(input uint64, expected string) {
				Expect(formatBytes(input)).To(Equal(expected))
			},
			Entry("sub-unit stays as bytes", uint64(512), "512b"),
			Entry("exact kibibyte", uint64(1024), "1.0kb"),
			Entry("exact mebibyte", uint64(1<<20), "1.0mb"),
			Entry("exact gibibyte", uint64(1<<30), "1.0gb"),
			Entry("fractional kibibyte", uint64(1536), "1.5kb"),
		)
	})

	// -------------------------------------------------------------------------
	// buildNodesText
	// -------------------------------------------------------------------------

	Describe("buildNodesText", func() {
		It("marks the elected cluster manager with a *", func() {
			out := buildNodesText([]responses.CatNodesResponse{
				{Name: "node-a", Ip: "10.0.0.1", ClusterManager: "*"},
				{Name: "node-b", Ip: "10.0.0.2", ClusterManager: "-"},
			}, 2, 2, DefaultLimits())
			Expect(out).To(ContainSubstring("node-a *"))
			Expect(out).To(ContainSubstring("(* = elected cluster manager)"))
			Expect(out).To(ContainSubstring("node-b"))
			Expect(out).NotTo(ContainSubstring("node-b *"))
		})

		It("falls back to the legacy master column for the marker", func() {
			out := buildNodesText([]responses.CatNodesResponse{
				{Name: "old-master", Master: "*"},
			}, 1, 1, DefaultLimits())
			Expect(out).To(ContainSubstring("old-master *"))
		})

		It("caps the number of nodes shown", func() {
			var nodes []responses.CatNodesResponse
			for i := 0; i < maxNodesShown+7; i++ {
				nodes = append(nodes, responses.CatNodesResponse{Name: fmt.Sprintf("node-%d", i)})
			}
			out := buildNodesText(nodes, maxNodesShown+7, maxNodesShown+7, DefaultLimits())
			Expect(out).To(ContainSubstring("... and 7 more nodes"))
			Expect(strings.Count(out, "node-")).To(Equal(maxNodesShown))
		})

		It("shows every node and no truncation footer under UnlimitedLimits", func() {
			total := maxNodesShown + 7
			var nodes []responses.CatNodesResponse
			for i := 0; i < total; i++ {
				nodes = append(nodes, responses.CatNodesResponse{Name: fmt.Sprintf("node-%d", i)})
			}
			out := buildNodesText(nodes, total, total, UnlimitedLimits())
			Expect(out).NotTo(ContainSubstring("more nodes"))
			Expect(strings.Count(out, "node-")).To(Equal(total))
		})
	})

	// -------------------------------------------------------------------------
	// buildProblemIndicesText
	// -------------------------------------------------------------------------

	Describe("buildProblemIndicesText", func() {
		It("reports (none) when every index is green", func() {
			out := buildProblemIndicesText(map[string]responses.IndexHealth{
				"logs": {Status: "green"},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring("=== NON-GREEN INDICES ==="))
			Expect(out).To(ContainSubstring("(none)"))
		})

		It("lists non-green indices with red before yellow", func() {
			out := buildProblemIndicesText(map[string]responses.IndexHealth{
				"green-idx":  {Status: "green"},
				"yellow-idx": {Status: "yellow", NumberOfShards: 1, ActivePrimaryShards: 1, UnassignedShards: 1},
				"red-idx":    {Status: "red", NumberOfShards: 2, ActivePrimaryShards: 1, UnassignedShards: 1},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring("red-idx"))
			Expect(out).To(ContainSubstring("yellow-idx"))
			Expect(out).NotTo(ContainSubstring("green-idx"))
			Expect(strings.Index(out, "red-idx")).To(BeNumerically("<", strings.Index(out, "yellow-idx")))
		})

		It("caps the number of problem indices shown", func() {
			indices := map[string]responses.IndexHealth{}
			for i := 0; i < maxProblemIndices+3; i++ {
				indices[fmt.Sprintf("idx-%02d", i)] = responses.IndexHealth{Status: "yellow"}
			}
			out := buildProblemIndicesText(indices, DefaultLimits())
			Expect(out).To(ContainSubstring("... and 3 more non-green indices"))
		})
	})

	// -------------------------------------------------------------------------
	// buildIndicesText
	// -------------------------------------------------------------------------

	Describe("buildIndicesText", func() {
		It("shows the top-N header when a limit is set", func() {
			out := buildIndicesText([]responses.CatIndicesResponse{
				{Index: "big", StoreSize: "100mb", Health: "green", Status: "open"},
				{Index: "small", StoreSize: "1mb", Health: "green", Status: "open"},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring(fmt.Sprintf("Top %d by size", maxIndicesShown)))
		})

		It("shows 'all, by size' header under UnlimitedLimits", func() {
			out := buildIndicesText([]responses.CatIndicesResponse{
				{Index: "a", StoreSize: "5mb"},
			}, UnlimitedLimits())
			Expect(out).To(ContainSubstring("all, by size"))
		})

		It("sorts indices largest-first", func() {
			out := buildIndicesText([]responses.CatIndicesResponse{
				{Index: "small", StoreSize: "1mb"},
				{Index: "large", StoreSize: "500mb"},
				{Index: "medium", StoreSize: "50mb"},
			}, UnlimitedLimits())
			Expect(strings.Index(out, "large")).To(BeNumerically("<", strings.Index(out, "medium")))
			Expect(strings.Index(out, "medium")).To(BeNumerically("<", strings.Index(out, "small")))
		})

		It("renders expected column headers", func() {
			out := buildIndicesText(nil, DefaultLimits())
			Expect(out).To(ContainSubstring("HEALTH"))
			Expect(out).To(ContainSubstring("INDEX"))
			Expect(out).To(ContainSubstring("SIZE"))
		})
	})

	// -------------------------------------------------------------------------
	// buildShardsText — inline allocation explain
	// -------------------------------------------------------------------------

	Describe("buildShardsText inline allocation explain", func() {
		It("inlines the reason, explanation and only blocking deciders for a matching shard", func() {
			shards := []responses.CatShardsResponse{
				{Index: "research_papers", Shard: "0", PrimaryOrReplica: "r", State: "UNASSIGNED"},
			}
			out := buildShardsText(shards, []responses.AllocationExplainResponse{
				{
					Index:               "research_papers",
					Shard:               0,
					Primary:             false,
					CurrentState:        "unassigned",
					CanAllocate:         "no",
					AllocateExplanation: "cannot allocate to any of the nodes",
					UnassignedInfo:      &responses.UnassignedInfo{Reason: "CLUSTER_RECOVERED"},
					NodeAllocationDecisions: []responses.AllocationExplainNodeDecision{
						{
							Name:     "node1",
							Decision: "no",
							Deciders: []responses.AllocationDecision{
								{Decider: "same_shard", Decision: "NO", Explanation: "copy already allocated"},
								{Decider: "disk_threshold", Decision: "YES", Explanation: "enough disk"},
							},
						},
					},
				},
			}, DefaultLimits(), false)
			Expect(out).To(ContainSubstring("[research_papers] shard 0"))
			Expect(out).To(ContainSubstring("reason:      CLUSTER_RECOVERED"))
			Expect(out).To(ContainSubstring("cannot allocate to any of the nodes"))
			Expect(out).To(ContainSubstring("same_shard"))
			// YES deciders are noise and must be omitted.
			Expect(out).NotTo(ContainSubstring("disk_threshold"))
		})

		It("caps the number of candidate nodes shown per shard", func() {
			var decisions []responses.AllocationExplainNodeDecision
			for i := 0; i < maxDeciderNodes+4; i++ {
				decisions = append(decisions, responses.AllocationExplainNodeDecision{
					Name:     fmt.Sprintf("node-%d", i),
					Decision: "no",
					Deciders: []responses.AllocationDecision{{Decider: "awareness", Decision: "NO", Explanation: "x"}},
				})
			}
			out := buildShardsText(
				[]responses.CatShardsResponse{{Index: "i", Shard: "0", PrimaryOrReplica: "p", State: "UNASSIGNED"}},
				[]responses.AllocationExplainResponse{{Index: "i", Shard: 0, Primary: true, NodeAllocationDecisions: decisions}},
				DefaultLimits(), false,
			)
			Expect(out).To(ContainSubstring("... and 4 more candidate nodes"))
		})

		It("falls back to the cat/shards reason when no explain is available", func() {
			out := buildShardsText([]responses.CatShardsResponse{
				{Index: "orders", Shard: "1", PrimaryOrReplica: "p", State: "UNASSIGNED", UnassignedReason: "NODE_LEFT"},
			}, nil, DefaultLimits(), false)
			Expect(out).To(ContainSubstring("reason: NODE_LEFT"))
		})
	})

	// -------------------------------------------------------------------------
	// buildShardsText — general
	// -------------------------------------------------------------------------

	Describe("buildShardsText", func() {
		// Regression: _cat/shards is requested with h=unassigned.reason, so the
		// field must unmarshal from the "unassigned.reason" JSON key.
		It("renders the unassigned reason from the unassigned.reason key", func() {
			var shards []responses.CatShardsResponse
			Expect(json.Unmarshal([]byte(`[
				{"index":"orders","shard":"0","prirep":"p","state":"STARTED"},
				{"index":"orders","shard":"1","prirep":"p","state":"UNASSIGNED","unassigned.reason":"NODE_LEFT"}
			]`), &shards)).To(Succeed())
			Expect(shards[1].UnassignedReason).To(Equal("NODE_LEFT"))

			out := buildShardsText(shards, nil, DefaultLimits(), false)
			Expect(out).To(ContainSubstring("NODE_LEFT"))
		})

		It("shows every unassigned shard and no truncation footer under UnlimitedLimits", func() {
			total := maxUnassignedShards + 5
			var shards []responses.CatShardsResponse
			for i := 0; i < total; i++ {
				shards = append(shards, responses.CatShardsResponse{
					Index: fmt.Sprintf("idx-%02d", i), Shard: "0", PrimaryOrReplica: "p",
					State: "UNASSIGNED", UnassignedReason: "NODE_LEFT",
				})
			}
			out := buildShardsText(shards, nil, UnlimitedLimits(), false)
			Expect(out).NotTo(ContainSubstring("not shown"))
			Expect(strings.Count(out, "[idx-")).To(Equal(total))
		})
	})

	// -------------------------------------------------------------------------
	// buildRecoveryText
	// -------------------------------------------------------------------------

	Describe("buildRecoveryText", func() {
		It("reports (none) when there are no active recoveries", func() {
			Expect(buildRecoveryText(nil, DefaultLimits())).To(ContainSubstring("(none)"))
		})

		It("caps the number of recoveries shown", func() {
			var rec []responses.CatRecoveryResponse
			for i := 0; i < maxRecoveryRows+5; i++ {
				rec = append(rec, responses.CatRecoveryResponse{Index: fmt.Sprintf("idx-%d", i), Shard: "0"})
			}
			out := buildRecoveryText(rec, DefaultLimits())
			Expect(out).To(ContainSubstring("... and 5 more active recoveries"))
		})
	})

	// -------------------------------------------------------------------------
	// buildAllocationText
	// -------------------------------------------------------------------------

	Describe("buildAllocationText", func() {
		It("renders the section header and column headers", func() {
			out := buildAllocationText([]responses.CatAllocationResponse{
				{Node: "n1", Shards: "10", DiskUsed: "1gb", DiskAvail: "9gb", DiskTotal: "10gb", DiskPercent: "10"},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring("=== ALLOCATION ==="))
			Expect(out).To(ContainSubstring("SHARDS"))
			Expect(out).To(ContainSubstring("DISK.USED"))
			Expect(out).To(ContainSubstring("n1"))
		})

		It("emits a truncation footer when rows exceed the limit", func() {
			var rows []responses.CatAllocationResponse
			for i := 0; i < maxAllocationRows+2; i++ {
				rows = append(rows, responses.CatAllocationResponse{Node: fmt.Sprintf("node-%d", i)})
			}
			out := buildAllocationText(rows, DefaultLimits())
			Expect(out).To(ContainSubstring("... and 2 more nodes"))
		})

		It("shows all rows and no footer under UnlimitedLimits", func() {
			var rows []responses.CatAllocationResponse
			for i := 0; i < maxAllocationRows+3; i++ {
				rows = append(rows, responses.CatAllocationResponse{Node: fmt.Sprintf("n%d", i)})
			}
			out := buildAllocationText(rows, UnlimitedLimits())
			Expect(out).NotTo(ContainSubstring("more nodes"))
		})
	})

	// -------------------------------------------------------------------------
	// buildThreadPoolText
	// -------------------------------------------------------------------------

	Describe("buildThreadPoolText", func() {
		It("reports (none) when no pool is backed up or rejecting", func() {
			out := buildThreadPoolText([]responses.CatThreadPoolResponse{
				{NodeName: "n1", Name: "write", Active: "1", Queue: "0", Rejected: "0"},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring("(none)"))
		})

		It("shows only pools with queue or rejected > 0, worst first", func() {
			out := buildThreadPoolText([]responses.CatThreadPoolResponse{
				{NodeName: "n1", Name: "search", Queue: "5", Rejected: "0"},
				{NodeName: "n2", Name: "write", Queue: "1", Rejected: "9"},
				{NodeName: "n3", Name: "get", Queue: "0", Rejected: "0"},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring("search"))
			Expect(out).To(ContainSubstring("write"))
			Expect(out).NotTo(ContainSubstring("get"))
			// write has more rejections, so it sorts above search.
			Expect(strings.Index(out, "write")).To(BeNumerically("<", strings.Index(out, "search")))
		})
	})

	// -------------------------------------------------------------------------
	// buildPendingTasksText
	// -------------------------------------------------------------------------

	Describe("buildPendingTasksText", func() {
		It("reports (none) when the queue is empty", func() {
			Expect(buildPendingTasksText(nil, DefaultLimits())).To(ContainSubstring("(none)"))
		})

		It("caps the number of tasks shown", func() {
			var tasks []responses.CatPendingTasksResponse
			for i := 0; i < maxPendingTasks+2; i++ {
				tasks = append(tasks, responses.CatPendingTasksResponse{InsertOrder: fmt.Sprintf("%d", i), Source: "put-mapping"})
			}
			out := buildPendingTasksText(tasks, DefaultLimits())
			Expect(out).To(ContainSubstring("... and 2 more pending tasks"))
		})
	})

	// -------------------------------------------------------------------------
	// buildBreakersText
	// -------------------------------------------------------------------------

	Describe("buildBreakersText", func() {
		It("reports (none) when no breaker is under pressure (utilisation < 95%)", func() {
			out := buildBreakersText(responses.NodesStatsResponse{
				Nodes: map[string]responses.NodeStatResponse{
					// 88% utilisation — below the 95% threshold, even though tripped > 0
					"id1": {Name: "n1", Breakers: map[string]responses.NodeStatBreakers{
						"parent": {Tripped: 5, LimitSizeInBytes: 1 << 30, EstimatedSizeInBytes: 900 << 20},
					}},
				},
			})
			Expect(out).To(ContainSubstring("(none)"))
		})

		It("lists breakers at >= 95% utilisation and shows UTIL% column", func() {
			out := buildBreakersText(responses.NodesStatsResponse{
				Nodes: map[string]responses.NodeStatResponse{
					"id1": {Name: "n1", Breakers: map[string]responses.NodeStatBreakers{
						// 97% — above threshold
						"parent": {Tripped: 3, LimitSizeInBytes: 1 << 30, EstimatedSizeInBytes: 990 << 20},
						// 10% — below threshold, must be omitted
						"fielddata": {Tripped: 1, LimitSizeInBytes: 1 << 30, EstimatedSizeInBytes: 100 << 20},
					}},
				},
			})
			Expect(out).To(ContainSubstring("parent"))
			Expect(out).To(ContainSubstring("n1"))
			Expect(out).To(ContainSubstring("UTIL%"))
			Expect(out).NotTo(ContainSubstring("fielddata"))
		})
	})

	// -------------------------------------------------------------------------
	// buildClusterManagerText
	// -------------------------------------------------------------------------

	Describe("buildClusterManagerText", func() {
		It("renders the section header, column headers, and node data", func() {
			out := buildClusterManagerText([]responses.CatClusterManagerResponse{
				{Id: "abc123", Host: "10.0.0.1", Ip: "10.0.0.1", Node: "manager-node"},
			})
			Expect(out).To(ContainSubstring("=== Cluster Manager ==="))
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("NODE"))
			Expect(out).To(ContainSubstring("manager-node"))
			Expect(out).To(ContainSubstring("abc123"))
		})

		It("renders an empty table without panicking when no manager is present", func() {
			out := buildClusterManagerText(nil)
			Expect(out).To(ContainSubstring("=== Cluster Manager ==="))
			Expect(strings.Count(out, "\n")).To(BeNumerically(">=", 1))
		})
	})

	// -------------------------------------------------------------------------
	// buildSegmentsText
	// -------------------------------------------------------------------------

	Describe("buildSegmentsText", func() {
		It("shows the top-N header when a limit is set", func() {
			out := buildSegmentsText([]responses.CatSegmentsResponse{
				{Index: "a", Size: "10mb"},
			}, DefaultLimits())
			Expect(out).To(ContainSubstring(fmt.Sprintf("Top %d by size", maxSegmentsShown)))
		})

		It("shows 'all, by size' header under UnlimitedLimits", func() {
			out := buildSegmentsText([]responses.CatSegmentsResponse{
				{Index: "a", Size: "10mb"},
			}, UnlimitedLimits())
			Expect(out).To(ContainSubstring("all, by size"))
		})

		It("sorts segments largest-first", func() {
			out := buildSegmentsText([]responses.CatSegmentsResponse{
				{Index: "tiny", Size: "1kb"},
				{Index: "huge", Size: "1gb"},
				{Index: "mid", Size: "1mb"},
			}, UnlimitedLimits())
			Expect(strings.Index(out, "huge")).To(BeNumerically("<", strings.Index(out, "mid")))
			Expect(strings.Index(out, "mid")).To(BeNumerically("<", strings.Index(out, "tiny")))
		})

		It("renders all expected column headers", func() {
			out := buildSegmentsText(nil, DefaultLimits())
			for _, col := range []string{"INDEX", "SHARD", "SIZE", "COMPOUND", "COMMITTED"} {
				Expect(out).To(ContainSubstring(col))
			}
		})
	})

	// -------------------------------------------------------------------------
	// BuildStatusText
	// -------------------------------------------------------------------------

	Describe("BuildStatusText", func() {
		It("returns a single error line when every source failed", func() {
			boom := fmt.Errorf("boom")
			errs := ClusterDataErr{
				healthErr: boom, nodesErr: boom, indicesErr: boom, shardsErr: boom,
				allocationErr: boom, clusterManagerErr: boom, segmentsErr: boom,
				pendingTasksErr: boom, threadPoolErr: boom, recoveryErr: boom, nodesStatsErr: boom,
			}
			out := BuildStatusText("c1", ClusterData{}, errs)
			Expect(out).To(ContainSubstring("could not fetch any cluster data"))
		})

		It("does not emit a separate allocation explain section (now inlined per shard)", func() {
			data := ClusterData{health: responses.ClusterHealthResponse{Status: "green"}}
			out := BuildStatusText("c1", data, ClusterDataErr{})
			Expect(out).To(ContainSubstring("=== CLUSTER HEALTH ==="))
			Expect(out).NotTo(ContainSubstring("ALLOCATION EXPLAIN"))
		})
	})

	// -------------------------------------------------------------------------
	// BuildStatusTextWithLimits
	// -------------------------------------------------------------------------

	Describe("BuildStatusTextWithLimits", func() {
		It("returns an error banner when all sources fail", func() {
			boom := fmt.Errorf("boom")
			e := ClusterDataErr{
				healthErr: boom, nodesErr: boom, indicesErr: boom, shardsErr: boom,
				allocationErr: boom, clusterManagerErr: boom, segmentsErr: boom,
				pendingTasksErr: boom, threadPoolErr: boom, recoveryErr: boom, nodesStatsErr: boom,
			}
			out := BuildStatusTextWithLimits("mycluster", ClusterData{}, e, DefaultLimits())
			Expect(out).To(ContainSubstring("could not fetch any cluster data"))
		})

		It("renders an error line per failed section when some sources succeed", func() {
			boom := fmt.Errorf("connection refused")
			e := ClusterDataErr{nodesErr: boom, indicesErr: boom}
			data := ClusterData{health: responses.ClusterHealthResponse{Status: "green"}}
			out := BuildStatusTextWithLimits("mycluster", data, e, DefaultLimits())
			Expect(out).To(ContainSubstring("ERROR: could not fetch nodes"))
			Expect(out).To(ContainSubstring("ERROR: could not fetch indices"))
			Expect(out).To(ContainSubstring("=== CLUSTER HEALTH ==="))
		})

		It("includes all section headers for a fully healthy cluster", func() {
			data := ClusterData{
				health:         responses.ClusterHealthResponse{Status: "green"},
				nodes:          []responses.CatNodesResponse{{Name: "n1"}},
				clusterManager: []responses.CatClusterManagerResponse{{Node: "n1"}},
				indices:        []responses.CatIndicesResponse{{Index: "i1", StoreSize: "1mb"}},
				allocation:     []responses.CatAllocationResponse{{Node: "n1"}},
				threadPool:     []responses.CatThreadPoolResponse{{Name: "write", Queue: "0", Rejected: "0"}},
				recovery:       []responses.CatRecoveryResponse{},
				segments:       []responses.CatSegmentsResponse{{Index: "i1", Size: "1mb"}},
			}
			out := BuildStatusTextWithLimits("mycluster", data, ClusterDataErr{}, DefaultLimits())
			Expect(out).To(ContainSubstring("=== OVERVIEW ==="))
			Expect(out).To(ContainSubstring("Cluster: mycluster"))
			Expect(out).To(ContainSubstring("=== CLUSTER HEALTH ==="))
			Expect(out).To(ContainSubstring("=== NODES ==="))
			Expect(out).To(ContainSubstring("=== Cluster Manager ==="))
			Expect(out).To(ContainSubstring("=== INDICES"))
			Expect(out).To(ContainSubstring("=== ALLOCATION ==="))
			Expect(out).To(ContainSubstring("=== SEGMENTS"))
		})

		It("respects custom limits passed by the caller", func() {
			var nodes []responses.CatNodesResponse
			for i := 0; i < 5; i++ {
				nodes = append(nodes, responses.CatNodesResponse{Name: fmt.Sprintf("node-%d", i)})
			}
			custom := DefaultLimits()
			custom.Nodes = 2
			out := BuildStatusTextWithLimits("c", ClusterData{
				health: responses.ClusterHealthResponse{Status: "green"},
				nodes:  nodes,
			}, ClusterDataErr{}, custom)
			Expect(out).To(ContainSubstring("... and 3 more nodes"))
		})
	})

	// -------------------------------------------------------------------------
	// BuildProblemsText
	// -------------------------------------------------------------------------

	Describe("BuildProblemsText", func() {
		It("returns 'No problems detected' for a fully green, quiet cluster", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{Status: "green"},
			}, ClusterDataErr{})
			Expect(out).To(Equal("No problems detected.\n"))
		})

		It("returns an error line when every data source failed", func() {
			boom := fmt.Errorf("boom")
			errs := ClusterDataErr{
				healthErr: boom, nodesErr: boom, indicesErr: boom, shardsErr: boom,
				allocationErr: boom, clusterManagerErr: boom, segmentsErr: boom,
				pendingTasksErr: boom, threadPoolErr: boom, recoveryErr: boom, nodesStatsErr: boom,
			}
			out := BuildProblemsText("c1", ClusterData{}, errs)
			Expect(out).To(ContainSubstring("could not fetch any cluster data"))
		})

		It("includes health section and non-green indices when status is yellow", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{
					Status:           "yellow",
					UnassignedShards: 2,
					Indices: map[string]responses.IndexHealth{
						"bad-index": {Status: "yellow", NumberOfShards: 1, ActivePrimaryShards: 1, UnassignedShards: 2},
					},
				},
			}, ClusterDataErr{})
			Expect(out).To(ContainSubstring("Cluster: mycluster"))
			Expect(out).To(ContainSubstring("=== CLUSTER HEALTH ==="))
			Expect(out).To(ContainSubstring("yellow"))
			Expect(out).To(ContainSubstring("=== NON-GREEN INDICES ==="))
			Expect(out).To(ContainSubstring("bad-index"))
		})

		It("returns no problems when cluster is green with only assigned shards", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{Status: "green"},
				shards: []responses.CatShardsResponse{
					{Index: "ok", Shard: "0", PrimaryOrReplica: "p", State: "STARTED"},
				},
			}, ClusterDataErr{})
			Expect(out).To(Equal("No problems detected.\n"))
		})

		It("includes the shards section when there are unassigned shards", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{Status: "yellow", UnassignedShards: 1},
				shards: []responses.CatShardsResponse{
					{Index: "my-idx", Shard: "0", PrimaryOrReplica: "r", State: "UNASSIGNED", UnassignedReason: "NODE_LEFT"},
				},
			}, ClusterDataErr{})
			Expect(out).To(ContainSubstring("=== UNASSIGNED SHARDS ==="))
			Expect(out).To(ContainSubstring("my-idx"))
			Expect(out).To(ContainSubstring("NODE_LEFT"))
		})

		It("includes the thread pool section when a pool is backed up", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health:     responses.ClusterHealthResponse{Status: "green"},
				threadPool: []responses.CatThreadPoolResponse{{NodeName: "n1", Name: "write", Queue: "10", Rejected: "5"}},
			}, ClusterDataErr{})
			Expect(out).To(ContainSubstring("=== THREAD POOLS"))
			Expect(out).To(ContainSubstring("write"))
		})

		It("includes the pending tasks section when tasks are queued", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health:       responses.ClusterHealthResponse{Status: "green"},
				pendingTasks: []responses.CatPendingTasksResponse{{InsertOrder: "1", Source: "put-mapping"}},
			}, ClusterDataErr{})
			Expect(out).To(ContainSubstring("=== PENDING TASKS ==="))
			Expect(out).To(ContainSubstring("put-mapping"))
		})

		It("includes the circuit breakers section when a breaker is >= 95% utilised", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{Status: "green"},
				nodesStats: responses.NodesStatsResponse{
					Nodes: map[string]responses.NodeStatResponse{
						"id1": {Name: "n1", Breakers: map[string]responses.NodeStatBreakers{
							// 97% utilisation
							"parent": {Tripped: 2, LimitSizeInBytes: 1 << 30, EstimatedSizeInBytes: 990 << 20},
						}},
					},
				},
			}, ClusterDataErr{})
			Expect(out).To(ContainSubstring("=== CIRCUIT BREAKERS (>= 95% utilisation) ==="))
			Expect(out).To(ContainSubstring("parent"))
		})

		It("does not include circuit breakers section when utilisation is below 95% even if tripped > 0", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{Status: "green"},
				nodesStats: responses.NodesStatsResponse{
					Nodes: map[string]responses.NodeStatResponse{
						// 88% utilisation
						"id1": {Name: "n1", Breakers: map[string]responses.NodeStatBreakers{
							"parent": {Tripped: 5, LimitSizeInBytes: 1 << 30, EstimatedSizeInBytes: 900 << 20},
						}},
					},
				},
			}, ClusterDataErr{})
			Expect(out).To(Equal("No problems detected.\n"))
		})

		It("does not include sections with no problems", func() {
			out := BuildProblemsText("mycluster", ClusterData{
				health: responses.ClusterHealthResponse{Status: "yellow", UnassignedShards: 1},
				shards: []responses.CatShardsResponse{
					{Index: "my-idx", Shard: "0", PrimaryOrReplica: "r", State: "UNASSIGNED"},
				},
				threadPool: []responses.CatThreadPoolResponse{
					{NodeName: "n1", Name: "search", Queue: "0", Rejected: "0"},
				},
			}, ClusterDataErr{})
			Expect(out).NotTo(ContainSubstring("=== THREAD POOLS"))
			Expect(out).NotTo(ContainSubstring("=== PENDING TASKS"))
			Expect(out).NotTo(ContainSubstring("=== CIRCUIT BREAKERS"))
		})
	})
})
