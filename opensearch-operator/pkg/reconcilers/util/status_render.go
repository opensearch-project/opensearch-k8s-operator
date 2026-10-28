package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

// deciderDescriptions maps allocation decider names to a short human-readable
// explanation shown in the problems key only.
var deciderDescriptions = map[string]string{
	"same_shard":                   "Prevents a primary shard and its replica from being allocated to the same node.",
	"disk_threshold":               "Checks whether the node has sufficient disk space based on low/high watermark thresholds.",
	"filter":                       "Applies allocation filters (index.routing.allocation.include/exclude/require).",
	"awareness":                    "Enforces shard allocation awareness across node attributes such as availability zones or racks.",
	"enable":                       "Checks whether shard allocation is enabled at the cluster, index, or shard level.",
	"throttling":                   "Limits the number of concurrent shard recoveries (cluster.routing.allocation.node_concurrent_recoveries).",
	"shards_limit":                 "Enforces the maximum number of shards per node or per index.",
	"max_retry":                    "Prevents repeated allocation attempts for shards that have already failed multiple times.",
	"node_version":                 "Ensures shards are only allocated to nodes with a compatible OpenSearch version.",
	"snapshot_in_progress":         "Prevents shard allocation while a snapshot operation is in progress for that shard.",
	"restore_in_progress":          "Controls shard allocation during restoration from a snapshot.",
	"rebalance_only_when_active":   "Prevents rebalancing when not all primary and replica shard copies are active.",
	"cluster_rebalance":            "Controls when rebalancing is allowed (cluster.routing.allocation.allow_rebalance).",
	"replica_after_primary_active": "Ensures replica shards are only allocated after their primary is active.",
}

// BuildStatusText renders the diagnostics text written to the status ConfigMap,
// using DefaultLimits so the result stays under the ConfigMap size limit.
func BuildStatusText(
	clusterName string,
	data ClusterData,
	err ClusterDataErr,
) string {
	return BuildStatusTextWithLimits(clusterName, data, err, DefaultLimits())
}

// BuildProblemsText renders a compact summary of only the sections that signal a problem
func BuildProblemsText(
	clusterName string,
	data ClusterData,
	dataErr ClusterDataErr,
) string {
	if dataErr.allFailed() {
		return fmt.Sprintf("Cluster: %s\nERROR: could not fetch any cluster data\n", clusterName)
	}

	body := renderProblems(data, dataErr, DefaultLimits())
	if body == "" {
		return noProblemsDetected
	}

	// Prepend a small header so the output is self-identifying.
	var out strings.Builder
	fmt.Fprintf(&out, "\n=== OVERVIEW ===\n")
	fmt.Fprintf(&out, "Cluster: %s\n", clusterName)
	fmt.Fprintf(&out, "Last Updated: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	out.WriteString(body)
	return out.String()
}

// problemCounts is the count-only summary of active cluster problems
type problemCounts struct {
	busyThreads        int
	pendingTasks       int
	nodesUnderPressure int
}

// countProblems computes the problem summary counts without rendering any text.
func countProblems(data ClusterData, dataErr ClusterDataErr) problemCounts {
	var pc problemCounts

	if dataErr.threadPoolErr == nil {
		for _, t := range data.threadPool {
			q, _ := strconv.Atoi(t.Queue)
			rej, _ := strconv.Atoi(t.Rejected)
			if q > 0 || rej > 0 {
				pc.busyThreads++
			}
		}
	}

	if dataErr.pendingTasksErr == nil {
		pc.pendingTasks = len(data.pendingTasks)
	}

	if dataErr.nodesStatsErr == nil {
		for _, node := range data.nodesStats.Nodes {
			for _, br := range node.Breakers {
				if breakerUtilPct(br) >= 95 {
					pc.nodesUnderPressure++
					break
				}
			}
		}
	}

	return pc
}

// renderProblems builds the problems summary body (no header), returning "" when every check is clear
func renderProblems(data ClusterData, dataErr ClusterDataErr, limits Limits) string {
	pc := countProblems(data, dataErr)

	var b strings.Builder

	// --- cluster health / non-green indices ---
	if dataErr.healthErr != nil {
		fmt.Fprintf(&b, "ERROR: could not fetch cluster health: %v\n\n", dataErr.healthErr)
	} else if data.health.Status != "green" {
		fmt.Fprintf(&b, "=== CLUSTER HEALTH ===\n")
		w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "Status:\t%s\n", data.health.Status)
		fmt.Fprintf(w, "Unassigned Shards:\t%d\n", data.health.UnassignedShards)
		w.Flush()
		fmt.Fprintf(&b, "\n")
		b.WriteString(buildProblemIndicesText(data.health.Indices, limits))
	}

	// --- unassigned shards (detail + allocation explain) ---
	if dataErr.shardsErr != nil {
		fmt.Fprintf(&b, "ERROR: could not fetch shards: %v\n\n", dataErr.shardsErr)
	} else if len(unassignedSorted(data.shards)) > 0 {
		// showDeciderWhy=true: the problems key includes the decider descriptions.
		b.WriteString(buildShardsText(data.shards, data.allocationExplain, limits, true))
	}

	// --- thread pools with queue / rejections ---
	if dataErr.threadPoolErr != nil {
		fmt.Fprintf(&b, "ERROR: could not fetch thread pools: %v\n\n", dataErr.threadPoolErr)
	} else if pc.busyThreads > 0 {
		b.WriteString(buildThreadPoolText(data.threadPool, limits))
	}

	// --- pending cluster-manager tasks ---
	if dataErr.pendingTasksErr != nil {
		fmt.Fprintf(&b, "ERROR: could not fetch pending tasks: %v\n\n", dataErr.pendingTasksErr)
	} else if pc.pendingTasks > 0 {
		b.WriteString(buildPendingTasksText(data.pendingTasks, limits))
	}

	// --- circuit breakers under pressure (utilisation >= 95%) ---
	if dataErr.nodesStatsErr != nil {
		fmt.Fprintf(&b, "ERROR: could not fetch node stats: %v\n\n", dataErr.nodesStatsErr)
	} else if pc.nodesUnderPressure > 0 {
		b.WriteString(buildBreakersText(data.nodesStats))
	}

	return b.String()
}

// BuildStatusTextWithLimits renders the diagnostics text with caller-supplied caps.
func BuildStatusTextWithLimits(
	clusterName string,
	data ClusterData,
	err ClusterDataErr,
	limits Limits,
) string {

	if err.allFailed() {
		return fmt.Sprintf("Cluster: %s\nERROR: could not fetch any cluster data\n", clusterName)
	}

	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "\n=== OVERVIEW ===\n")
	fmt.Fprintf(&b, "Cluster: %s\n", clusterName)
	fmt.Fprintf(&b, "Last Updated: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))

	// Diagnostics
	sections := []struct {
		err   error
		label string
		build func() string
	}{
		// Cluster Health (includes non-green indices)
		{err.healthErr, "cluster health", func() string { return buildHealthText(data.health, data, err, limits) }},
		// Cluster Nodes
		{err.nodesErr, "nodes", func() string {
			return buildNodesText(data.nodes, data.health.NumberOfNodes, data.health.NumberOfDataNodes, limits)
		}},
		// Cluster Manager
		{err.clusterManagerErr, "cluster manager", func() string { return buildClusterManagerText(data.clusterManager) }},
		// Cluster Indices
		{err.indicesErr, "indices", func() string { return buildIndicesText(data.indices, limits) }},
		// Cluster Shards + inline allocation explain for unassigned shards
		{err.shardsErr, "shards", func() string { return buildShardsText(data.shards, data.allocationExplain, limits, false) }},
		// In-progress shard recovery
		{err.recoveryErr, "recovery", func() string { return buildRecoveryText(data.recovery, limits) }},
		// Disk allocation per node
		{err.allocationErr, "allocation", func() string { return buildAllocationText(data.allocation, limits) }},
		// Thread pool backlog / rejections
		{err.threadPoolErr, "thread pool", func() string { return buildThreadPoolText(data.threadPool, limits) }},
		// Pending cluster-manager tasks
		{err.pendingTasksErr, "pending tasks", func() string { return buildPendingTasksText(data.pendingTasks, limits) }},
		// Tripped circuit breakers
		{err.nodesStatsErr, "circuit breakers", func() string { return buildBreakersText(data.nodesStats) }},
		// Cluster Segments
		{err.segmentsErr, "segments", func() string { return buildSegmentsText(data.segments, limits) }},
	}

	// Show an error placeholder for any section whose fetch failed.
	for _, s := range sections {
		if s.err != nil {
			fmt.Fprintf(&b, "ERROR: could not fetch %s: %v\n\n", s.label, s.err)
		} else if out := s.build(); out != "" {
			b.WriteString(out)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// buildHealthText renders the CLUSTER HEALTH and NON-GREEN INDICES sections
func buildHealthText(health responses.ClusterHealthResponse, data ClusterData, dataErr ClusterDataErr, limits Limits) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprintf(&b, "=== CLUSTER HEALTH ===\n")
	fmt.Fprintf(w, "Status:\t%s\n", health.Status)
	if health.UnassignedShards > 0 {
		fmt.Fprintf(w, "Unassigned Shards:\t%d\n", health.UnassignedShards)
	}

	// Count-only pass: no problem section is rendered here (renderProblems owns
	// that for the 'problems' key), so the health summary stays cheap.
	pc := countProblems(data, dataErr)
	if pc.busyThreads > 0 {
		fmt.Fprintf(w, "Busy Threads:\t%d\n", pc.busyThreads)
	}
	if pc.pendingTasks > 0 {
		fmt.Fprintf(w, "Pending Tasks:\t%d\n", pc.pendingTasks)
	}
	if pc.nodesUnderPressure > 0 {
		fmt.Fprintf(w, "Nodes Under Pressure:\t%d\n", pc.nodesUnderPressure)
	}

	w.Flush()

	if health.Status != "green" {
		fmt.Fprintf(&b, ">> See the 'problems' key in this ConfigMap for a detailed breakdown\n")
	}
	fmt.Fprintf(&b, "\n")

	b.WriteString(buildProblemIndicesText(health.Indices, limits))
	return b.String()
}

// buildProblemIndicesText lists indices whose health is not green
func buildProblemIndicesText(indices map[string]responses.IndexHealth, limits Limits) string {
	type problem struct {
		name      string
		status    string
		unRep     int
		unPrimary int
	}
	var problems []problem
	for name, h := range indices {
		if h.Status == "green" {
			continue
		}
		unPrimary := h.NumberOfShards - h.ActivePrimaryShards
		problems = append(problems, problem{name, h.Status, h.UnassignedShards, unPrimary})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== NON-GREEN INDICES ===\n")
	if len(problems) == 0 {
		fmt.Fprintf(&b, "(none)\n\n")
		return b.String()
	}

	// Red (unavailable primaries) before yellow (missing replicas), then by name.
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].status != problems[j].status {
			return problems[i].status == "red"
		}
		return problems[i].name < problems[j].name
	})
	shown := capCount(len(problems), limits.ProblemIndices)
	remaining := len(problems) - shown
	problems = problems[:shown]

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "INDEX\tHEALTH\tUNASSIGNED.PRIMARY\tUNASSIGNED.SHARDS\n")
	for _, p := range problems {
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", p.name, p.status, p.unPrimary, p.unRep)
	}
	w.Flush()
	if remaining > 0 {
		fmt.Fprintf(&b, "... and %d more non-green indices\n", remaining)
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// buildNodesText renders the NODES section with per-node heap, RAM, CPU and load metrics
func buildNodesText(nodes []responses.CatNodesResponse, numberOfNodes int, numberOfDataNodes int, limits Limits) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprintf(&b, "=== NODES ===\n")
	fmt.Fprintf(w, "Nodes:\t%d\n", numberOfNodes)
	fmt.Fprintf(w, "Data Nodes:\t%d\n", numberOfDataNodes)
	w.Flush()
	fmt.Fprintf(&b, "\n")

	w = tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tIP\tHEAP\tRAM\tCPU\tLOAD\tROLE\n")

	shown := capCount(len(nodes), limits.Nodes)
	remaining := len(nodes) - shown
	nodes = nodes[:shown]
	for _, n := range nodes {
		name := n.Name
		// Mark the elected cluster manager with a trailing "*".
		if n.ClusterManager == "*" || n.Master == "*" {
			name += " *"
		}
		fmt.Fprintf(w, "%s\t%s\t%s%%\t%s%%\t%s%%\t%s\t%s\n",
			name,
			n.Ip,
			n.HeapPercent,
			n.RamPercent,
			n.Cpu,
			n.Load1m,
			n.NodeRole,
		)
	}

	w.Flush()
	if remaining > 0 {
		fmt.Fprintf(&b, "... and %d more nodes\n", remaining)
	}
	fmt.Fprintf(&b, "(* = elected cluster manager)\n")
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// buildIndicesText renders the INDICES section, sorted by store size descending
func buildIndicesText(indices []responses.CatIndicesResponse, limits Limits) string {
	sort.Slice(indices, func(i, j int) bool {
		return parseSizeToBytes(indices[i].StoreSize) > parseSizeToBytes(indices[j].StoreSize)
	})
	indices = indices[:capCount(len(indices), limits.Indices)]
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	if limits.Indices > 0 {
		fmt.Fprintf(&b, "=== INDICES (Top %d by size) ===\n", limits.Indices)
	} else {
		fmt.Fprintf(&b, "=== INDICES (all, by size) ===\n")
	}
	fmt.Fprintf(w, "HEALTH\tSTATUS\tINDEX\tPRI\tREP\tDOCS\tSIZE\n")

	for _, idx := range indices {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			idx.Health,
			idx.Status,
			idx.Index,
			idx.Pri,
			idx.Rep,
			idx.DocsCount,
			idx.StoreSize,
		)
	}

	w.Flush()
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// parseSizeToBytes converts an OpenSearch size string (e.g. "120.3mb", "1.2gb") to bytes.
func parseSizeToBytes(s string) int64 {
	s = strings.ToLower(strings.TrimSpace(s))
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"tb", 1 << 40},
		{"gb", 1 << 30},
		{"mb", 1 << 20},
		{"kb", 1 << 10},
		{"b", 1},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			val, err := strconv.ParseFloat(strings.TrimSuffix(s, u.suffix), 64)
			if err != nil {
				return 0
			}
			return int64(val * float64(u.multiplier))
		}
	}
	return 0
}

// buildShardsText renders the shard summary and the unassigned-shard listing.
func buildShardsText(shards []responses.CatShardsResponse, explains []responses.AllocationExplainResponse, limits Limits, showDeciderWhy bool) string {

	unassigned := unassignedSorted(shards)

	var primary, replica, active, relocating, initializing int
	for _, s := range shards {
		if s.PrimaryOrReplica == "p" {
			primary++
		} else {
			replica++
		}
		switch s.State {
		case "STARTED":
			active++
		case "RELOCATING":
			relocating++
		case "INITIALIZING":
			initializing++
		}
	}

	// Build a lookup map from explain responses keyed by "index/shard/p|r".
	explainMap := make(map[string]responses.AllocationExplainResponse, len(explains))
	for _, e := range explains {
		prirep := "r"
		if e.Primary {
			prirep = "p"
		}
		key := fmt.Sprintf("%s/%s/%s", e.Index, strconv.Itoa(e.Shard), prirep)
		explainMap[key] = e
	}

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprintf(&b, "=== SHARDS ===\n")
	fmt.Fprintf(w, "Total Shards:\t%d\n", len(shards))
	fmt.Fprintf(w, "Primary Shards:\t%d\n", primary)
	fmt.Fprintf(w, "Replica Shards:\t%d\n", replica)
	fmt.Fprintf(w, "Active Shards:\t%d\n", active)
	fmt.Fprintf(w, "Relocating Shards:\t%d\n", relocating)
	fmt.Fprintf(w, "Initializing Shards:\t%d\n", initializing)
	fmt.Fprintf(w, "Unassigned Shards:\t%d\n", len(unassigned))
	w.Flush()
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "=== UNASSIGNED SHARDS ===\n")
	if len(unassigned) == 0 {
		fmt.Fprintf(&b, "(none)\n")
	} else {
		shown := capCount(len(unassigned), limits.UnassignedShards)
		display := unassigned[:shown]
		remaining := len(unassigned) - shown

		for _, s := range display {
			shardType := "replica"
			if s.PrimaryOrReplica == "p" {
				shardType = "primary"
			}

			fmt.Fprintf(&b, "[%s] shard %s · %s\n", s.Index, s.Shard, shardType)

			// Inline allocation explain if available for this shard.
			writeUnassignedShard(&b, s, explainMap, limits, showDeciderWhy)

			fmt.Fprintf(&b, "\n")
		}

		if remaining == 1 {
			fmt.Fprintf(&b, "... and %d more unassigned shard not shown\n", remaining)
		} else if remaining > 1 {
			fmt.Fprintf(&b, "... and %d more unassigned shards not shown\n", remaining)
		}
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// writeUnassignedShard writes the allocation-explain block for a single unassigned shard to b
func writeUnassignedShard(b *strings.Builder, s responses.CatShardsResponse, explainMap map[string]responses.AllocationExplainResponse, limits Limits, showDeciderWhy bool) {
	key := fmt.Sprintf("%s/%s/%s", s.Index, s.Shard, s.PrimaryOrReplica)
	if e, ok := explainMap[key]; ok {
		if e.UnassignedInfo != nil && e.UnassignedInfo.Reason != "" {
			fmt.Fprintf(b, "  reason:      %s\n", e.UnassignedInfo.Reason)
		}
		if e.AllocateExplanation != "" {
			fmt.Fprintf(b, "  explanation: %s\n", e.AllocateExplanation)
		}
		decisions := e.NodeAllocationDecisions
		shownNodes := capCount(len(decisions), limits.DeciderNodes)
		remainingNodes := len(decisions) - shownNodes
		decisions = decisions[:shownNodes]

		// Collect nodes that have at least one blocking decider.
		type blockedNode struct {
			name     string
			deciders []string
		}
		var blockedNodes []blockedNode
		for _, d := range decisions {
			var names []string
			for _, dec := range d.Deciders {
				if dec.Decision != "YES" {
					names = append(names, dec.Decider)
				}
			}
			if len(names) > 0 {
				blockedNodes = append(blockedNodes, blockedNode{d.Name, names})
			}
		}
		if len(blockedNodes) > 0 {
			fmt.Fprintf(b, "  blocked on:\n")
			for _, bn := range blockedNodes {
				for _, decider := range bn.deciders {
					fmt.Fprintf(b, "    %-40s  %s\n", bn.name, decider)
				}
			}
			if showDeciderWhy {

				// Deduplicate decider names that appeared in this shard's blocked list.
				seen := make(map[string]struct{})
				var unique []string
				for _, bn := range blockedNodes {
					for _, decider := range bn.deciders {
						if _, ok := seen[decider]; !ok {
							seen[decider] = struct{}{}
							unique = append(unique, decider)
						}
					}
				}
				sort.Strings(unique)
				fmt.Fprintf(b, "  why:\n")
				for _, decider := range unique {
					desc, ok := deciderDescriptions[decider]
					if !ok {
						desc = "No description available."
					}
					fmt.Fprintf(b, "    %s: %s\n", decider, desc)
				}
			}
		}
		if remainingNodes > 0 {
			fmt.Fprintf(b, "  ... and %d more candidate nodes\n", remainingNodes)
		}
	} else if s.UnassignedReason != "" {
		// No full explain available; fall back to the cat/shards reason code.
		fmt.Fprintf(b, "  reason: %s\n", s.UnassignedReason)
	}
}

// buildRecoveryText renders the ACTIVE RECOVERY section.
// limits.RecoveryRows caps the number of in-progress recoveries shown.
func buildRecoveryText(recovery []responses.CatRecoveryResponse, limits Limits) string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== ACTIVE RECOVERY ===\n")
	if len(recovery) == 0 {
		fmt.Fprintf(&b, "(none)\n\n")
		return b.String()
	}

	shown := capCount(len(recovery), limits.RecoveryRows)
	remaining := len(recovery) - shown
	recovery = recovery[:shown]

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "INDEX\tSHARD\tTYPE\tSTAGE\tSOURCE\tTARGET\tFILES%%\tBYTES%%\tTIME\n")
	for _, r := range recovery {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Index, r.Shard, r.Type, r.Stage, r.SourceNode, r.TargetNode,
			r.FilesPercent, r.BytesPercent, r.Time)
	}
	w.Flush()
	if remaining > 0 {
		fmt.Fprintf(&b, "... and %d more active recoveries\n", remaining)
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// buildAllocationText renders the ALLOCATION section showing disk usage per
// node. limits.AllocationRows caps the number of rows displayed.
func buildAllocationText(allocation []responses.CatAllocationResponse, limits Limits) string {
	shown := capCount(len(allocation), limits.AllocationRows)
	remaining := len(allocation) - shown
	allocation = allocation[:shown]

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprintf(&b, "=== ALLOCATION ===\n")
	fmt.Fprintf(w, "SHARDS\tDISK.INDICES\tDISK.USED\tDISK.AVAIL\tDISK.TOTAL\tDISK.PERCENT\tHOST\tIP\tNODE\n")

	for _, a := range allocation {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s%%\t%s\t%s\t%s\n",
			a.Shards,
			a.DiskIndices,
			a.DiskUsed,
			a.DiskAvail,
			a.DiskTotal,
			a.DiskPercent,
			a.Host,
			a.Ip,
			a.Node,
		)
	}

	w.Flush()
	if remaining > 0 {
		fmt.Fprintf(&b, "... and %d more nodes\n", remaining)
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// buildThreadPoolText shows only thread pools that are backed up or rejecting
// work (queue > 0 or rejected > 0); healthy pools are omitted as noise.
func buildThreadPoolText(threadPool []responses.CatThreadPoolResponse, limits Limits) string {
	var interesting []responses.CatThreadPoolResponse
	for _, t := range threadPool {
		queue, _ := strconv.Atoi(t.Queue)
		rejected, _ := strconv.Atoi(t.Rejected)
		if queue > 0 || rejected > 0 {
			interesting = append(interesting, t)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== THREAD POOLS (queue/rejected > 0) ===\n")
	if len(interesting) == 0 {
		fmt.Fprintf(&b, "(none)\n\n")
		return b.String()
	}

	// Most-rejected first, then most-queued, so the worst offenders are on top.
	sort.Slice(interesting, func(i, j int) bool {
		ri, _ := strconv.Atoi(interesting[i].Rejected)
		rj, _ := strconv.Atoi(interesting[j].Rejected)
		if ri != rj {
			return ri > rj
		}
		qi, _ := strconv.Atoi(interesting[i].Queue)
		qj, _ := strconv.Atoi(interesting[j].Queue)
		return qi > qj
	})
	shown := capCount(len(interesting), limits.ThreadPoolRows)
	remaining := len(interesting) - shown
	interesting = interesting[:shown]

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "NODE\tPOOL\tACTIVE\tQUEUE\tREJECTED\n")
	for _, t := range interesting {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.NodeName, t.Name, t.Active, t.Queue, t.Rejected)
	}
	w.Flush()
	if remaining > 0 {
		fmt.Fprintf(&b, "... and %d more thread pools\n", remaining)
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// buildPendingTasksText renders the PENDING TASKS section.
// limits.PendingTasks caps the number of rows displayed.
func buildPendingTasksText(tasks []responses.CatPendingTasksResponse, limits Limits) string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== PENDING TASKS ===\n")
	if len(tasks) == 0 {
		fmt.Fprintf(&b, "(none)\n\n")
		return b.String()
	}

	shown := capCount(len(tasks), limits.PendingTasks)
	remaining := len(tasks) - shown
	tasks = tasks[:shown]

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ORDER\tTIME.IN.QUEUE\tPRIORITY\tSOURCE\n")
	for _, t := range tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			t.InsertOrder, t.TimeInQueue, t.Priority, t.Source)
	}
	w.Flush()
	if remaining > 0 {
		fmt.Fprintf(&b, "... and %d more pending tasks\n", remaining)
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// breakerUtilPct returns the percentage of the breaker limit currently
// estimated to be in use. Returns 0 when the limit is 0 (disabled breaker).
func breakerUtilPct(br responses.NodeStatBreakers) float64 {
	if br.LimitSizeInBytes == 0 {
		return 0
	}
	return float64(br.EstimatedSizeInBytes) / float64(br.LimitSizeInBytes) * 100
}

// buildBreakersText lists circuit breakers under pressure (estimated memory ≥ 95% of the limit).
func buildBreakersText(stats responses.NodesStatsResponse) string {
	type row struct {
		node      string
		breaker   string
		util      float64
		tripped   uint32
		limit     uint64
		estimated uint64
	}
	var rows []row
	for _, node := range stats.Nodes {
		for name, br := range node.Breakers {
			if breakerUtilPct(br) >= 95 {
				rows = append(rows, row{node.Name, name, breakerUtilPct(br), br.Tripped, br.LimitSizeInBytes, br.EstimatedSizeInBytes})
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== CIRCUIT BREAKERS (>= 95%% utilisation) ===\n")
	if len(rows) == 0 {
		fmt.Fprintf(&b, "(none)\n\n")
		return b.String()
	}

	// Highest utilisation first, then by node/breaker for stable ordering.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].util != rows[j].util {
			return rows[i].util > rows[j].util
		}
		if rows[i].node != rows[j].node {
			return rows[i].node < rows[j].node
		}
		return rows[i].breaker < rows[j].breaker
	})

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "NODE\tBREAKER\tUTIL%%\tTRIPPED\tLIMIT\tESTIMATED\n")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%.1f%%\t%d\t%s\t%s\n",
			r.node, r.breaker, r.util, r.tripped, formatBytes(r.limit), formatBytes(r.estimated))
	}
	w.Flush()
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// formatBytes renders a byte count as a human-readable size using binary units.
func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%db", n)
	}
	div, exp := int64(unit), 0
	for size := n / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cb", float64(n)/float64(div), "kmgt"[exp])
}

// buildClusterManagerText renders the Cluster Manager section showing the
// current elected cluster manager node.
func buildClusterManagerText(clusterManager []responses.CatClusterManagerResponse) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprintf(&b, "=== Cluster Manager ===\n")
	fmt.Fprintf(w, "ID\tHOST\tIP\tNODE\n")

	for _, c := range clusterManager {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			c.Id,
			c.Host,
			c.Ip,
			c.Node,
		)
	}

	w.Flush()
	fmt.Fprintf(&b, "\n")
	return b.String()
}

// buildSegmentsText renders the SEGMENTS section, sorted by size descending so
// the largest segments appear first. limits.Segments caps the number of rows.
func buildSegmentsText(segments []responses.CatSegmentsResponse, limits Limits) string {

	sort.Slice(segments, func(i, j int) bool {
		return parseSizeToBytes(segments[i].Size) > parseSizeToBytes(segments[j].Size)
	})
	segments = segments[:capCount(len(segments), limits.Segments)]
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	if limits.Segments > 0 {
		fmt.Fprintf(&b, "=== SEGMENTS (Top %d by size) ===\n", limits.Segments)
	} else {
		fmt.Fprintf(&b, "=== SEGMENTS (all, by size) ===\n")
	}
	fmt.Fprintf(w, "INDEX\tSHARD\tPRIREP\tIP\tSEGMENT\tGENERATION\tDOCS.COUNT\tDOCS.DELETED\tSIZE\tSIZE.MEMORY\tCOMMITTED\tSEARCHABLE\tVERSION\tCOMPOUND\n")

	for _, s := range segments {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.Index,
			s.Shard,
			s.PrimaryOrReplica,
			s.Ip,
			s.Segment,
			s.Generation,
			s.DocsCount,
			s.DocsDeleted,
			s.Size,
			s.SizeMemory,
			s.Committed,
			s.Searchable,
			s.Version,
			s.Compound,
		)
	}

	w.Flush()
	fmt.Fprintf(&b, "\n")
	return b.String()
}
