package util

import (
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

// Per-section caps keep the rendered output well under the 1 MiB ConfigMap limit.
const (
	maxIndicesShown     = 10
	maxSegmentsShown    = 10
	maxUnassignedShards = 20
	maxNodesShown       = 50
	maxAllocationRows   = 50
	maxRecoveryRows     = 20
	maxThreadPoolRows   = 30
	maxPendingTasks     = 20
	maxProblemIndices   = 20
	// maxAllocationExplain also bounds the number of API calls made, not just output rows.
	maxAllocationExplain = 5
	// maxDeciderNodes bounds how many candidate nodes are shown per explained shard.
	maxDeciderNodes = 5
)

const (
	noProblemsDetected = "No problems detected.\n"
)

// Limits controls per-section row caps and the allocation/explain call budget.
// A field value <= 0 means no limit.
type Limits struct {
	Indices           int
	Segments          int
	UnassignedShards  int
	Nodes             int
	AllocationRows    int
	RecoveryRows      int
	ThreadPoolRows    int
	PendingTasks      int
	ProblemIndices    int
	AllocationExplain int
	DeciderNodes      int
}

// DefaultLimits returns the caps used for the status ConfigMap, keeping the
// rendered output well under the size limit even on large clusters.
func DefaultLimits() Limits {
	return Limits{
		Indices:           maxIndicesShown,
		Segments:          maxSegmentsShown,
		UnassignedShards:  maxUnassignedShards,
		Nodes:             maxNodesShown,
		AllocationRows:    maxAllocationRows,
		RecoveryRows:      maxRecoveryRows,
		ThreadPoolRows:    maxThreadPoolRows,
		PendingTasks:      maxPendingTasks,
		ProblemIndices:    maxProblemIndices,
		AllocationExplain: maxAllocationExplain,
		DeciderNodes:      maxDeciderNodes,
	}
}

// UnlimitedLimits returns limits with no caps on any section.
func UnlimitedLimits() Limits {
	return Limits{}
}

// capCount returns how many of n items to show given a limit. A limit <= 0 means
// unlimited, so all n are shown.
func capCount(n, limit int) int {
	if limit <= 0 || n < limit {
		return n
	}
	return limit
}

type ClusterData struct {
	health            responses.ClusterHealthResponse
	nodes             []responses.CatNodesResponse
	indices           []responses.CatIndicesResponse
	shards            []responses.CatShardsResponse
	allocation        []responses.CatAllocationResponse
	clusterManager    []responses.CatClusterManagerResponse
	segments          []responses.CatSegmentsResponse
	pendingTasks      []responses.CatPendingTasksResponse
	threadPool        []responses.CatThreadPoolResponse
	recovery          []responses.CatRecoveryResponse
	nodesStats        responses.NodesStatsResponse
	allocationExplain []responses.AllocationExplainResponse
}

type ClusterDataErr struct {
	healthErr            error
	nodesErr             error
	indicesErr           error
	shardsErr            error
	allocationErr        error
	clusterManagerErr    error
	segmentsErr          error
	pendingTasksErr      error
	threadPoolErr        error
	recoveryErr          error
	nodesStatsErr        error
	allocationExplainErr error
}

func NewClusterData(
	health responses.ClusterHealthResponse,
	nodes []responses.CatNodesResponse,
	indices []responses.CatIndicesResponse,
	shards []responses.CatShardsResponse,
	allocation []responses.CatAllocationResponse,
	clusterManager []responses.CatClusterManagerResponse,
	segments []responses.CatSegmentsResponse,
	pendingTasks []responses.CatPendingTasksResponse,
	threadPool []responses.CatThreadPoolResponse,
	recovery []responses.CatRecoveryResponse,
	nodesStats responses.NodesStatsResponse,
	allocationExplain []responses.AllocationExplainResponse,
) *ClusterData {
	return &ClusterData{
		health:            health,
		nodes:             nodes,
		indices:           indices,
		shards:            shards,
		allocation:        allocation,
		clusterManager:    clusterManager,
		segments:          segments,
		pendingTasks:      pendingTasks,
		threadPool:        threadPool,
		recovery:          recovery,
		nodesStats:        nodesStats,
		allocationExplain: allocationExplain,
	}
}

func NewClusterDataErr(
	healthErr error,
	nodesErr error,
	indicesErr error,
	shardsErr error,
	allocationErr error,
	clusterManagerErr error,
	segmentsErr error,
	pendingTasksErr error,
	threadPoolErr error,
	recoveryErr error,
	nodesStatsErr error,
	allocationExplainErr error,
) *ClusterDataErr {
	return &ClusterDataErr{
		healthErr:            healthErr,
		nodesErr:             nodesErr,
		indicesErr:           indicesErr,
		shardsErr:            shardsErr,
		allocationErr:        allocationErr,
		clusterManagerErr:    clusterManagerErr,
		segmentsErr:          segmentsErr,
		pendingTasksErr:      pendingTasksErr,
		threadPoolErr:        threadPoolErr,
		recoveryErr:          recoveryErr,
		nodesStatsErr:        nodesStatsErr,
		allocationExplainErr: allocationExplainErr,
	}
}

// allFailed reports whether every primary data source failed.
func (e ClusterDataErr) allFailed() bool {
	return e.healthErr != nil &&
		e.nodesErr != nil &&
		e.indicesErr != nil &&
		e.shardsErr != nil &&
		e.allocationErr != nil &&
		e.clusterManagerErr != nil &&
		e.segmentsErr != nil &&
		e.pendingTasksErr != nil &&
		e.threadPoolErr != nil &&
		e.recoveryErr != nil &&
		e.nodesStatsErr != nil
}
