package util

import (
	"sort"
	"strconv"

	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
)

// GatherClusterData collects all diagnostics data using DefaultLimits.
func GatherClusterData(client *services.OsClusterClient) (*ClusterData, *ClusterDataErr) {
	return GatherClusterDataWithLimits(client, DefaultLimits())
}

// GatherClusterDataWithLimits collects all diagnostics data. limits.AllocationExplain
// is the only limit that affects gathering (not just rendering): it caps the
// number of per-shard allocation/explain API calls made.
func GatherClusterDataWithLimits(client *services.OsClusterClient, limits Limits) (*ClusterData, *ClusterDataErr) {
	// level=indices populates per-index health so non-green indices can be listed
	clusterHealth, healthErr := client.GetHealth()
	clusterNodes, nodesErr := client.CatNodes()
	clusterIndices, indicesErr := client.CatIndices()
	clusterShards, shardsErr := client.CatShards([]string{"index", "shard", "prirep", "state", "unassigned.reason"})
	clusterAllocation, allocationErr := client.CatAllocation()
	clusterManager, clusterManagerErr := client.CatClusterManager()
	clusterSegments, segmentsErr := client.CatSegments()
	pendingTasks, pendingTasksErr := client.CatPendingTasks()
	threadPool, threadPoolErr := client.CatThreadPool()
	recovery, recoveryErr := client.CatRecovery()
	nodesStats, nodesStatsErr := client.NodesStats()

	// only explain shards already known to be unassigned, up to the cap
	allocationExplain, allocationExplainErr := GatherAllocationExplainWithLimit(client, clusterShards, shardsErr, limits.AllocationExplain)

	var clusterData = NewClusterData(
		clusterHealth,
		clusterNodes,
		clusterIndices,
		clusterShards,
		clusterAllocation,
		clusterManager,
		clusterSegments,
		pendingTasks,
		threadPool,
		recovery,
		nodesStats,
		allocationExplain,
	)

	var clusterDataErr = NewClusterDataErr(
		healthErr,
		nodesErr,
		indicesErr,
		shardsErr,
		allocationErr,
		clusterManagerErr,
		segmentsErr,
		pendingTasksErr,
		threadPoolErr,
		recoveryErr,
		nodesStatsErr,
		allocationExplainErr,
	)

	return clusterData, clusterDataErr
}

// GatherAllocationExplainWithLimit calls _cluster/allocation/explain for up to
// max unassigned shards. A max <= 0 removes the cap, which can be expensive
// when many shards are unassigned.
func GatherAllocationExplainWithLimit(
	client *services.OsClusterClient,
	shards []responses.CatShardsResponse,
	shardsErr error,
	max int,
) ([]responses.AllocationExplainResponse, error) {
	if shardsErr != nil {
		return nil, nil
	}

	unassigned := unassignedSorted(shards)
	if len(unassigned) == 0 {
		return nil, nil
	}
	unassigned = unassigned[:capCount(len(unassigned), max)]

	var explains []responses.AllocationExplainResponse
	var explainErr error
	for _, s := range unassigned {
		shardNum, _ := strconv.Atoi(s.Shard)
		primary := s.PrimaryOrReplica == "p"
		exp, err := client.GetAllocationExplain(s.Index, shardNum, primary)
		if err != nil {
			// keep going so one failed explain does not hide the others
			explainErr = err
			continue
		}
		explains = append(explains, exp)
	}
	return explains, explainErr
}

// unassignedSorted returns UNASSIGNED shards sorted by index name then numeric
// shard number. It is the single source of truth used by both the gather and
// render paths, so their ordering is always consistent.
func unassignedSorted(shards []responses.CatShardsResponse) []responses.CatShardsResponse {
	var unassigned []responses.CatShardsResponse
	for _, s := range shards {
		if s.State == "UNASSIGNED" {
			unassigned = append(unassigned, s)
		}
	}
	sort.Slice(unassigned, func(i, j int) bool {
		if unassigned[i].Index != unassigned[j].Index {
			return unassigned[i].Index < unassigned[j].Index
		}
		si, _ := strconv.Atoi(unassigned[i].Shard)
		sj, _ := strconv.Atoi(unassigned[j].Shard)
		return si < sj
	})
	return unassigned
}
