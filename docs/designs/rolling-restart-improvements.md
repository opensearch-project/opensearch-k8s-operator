# Rolling Restart Improvements for Multi-AZ Master Nodes

## Problem Statement

The OpenSearch Kubernetes operator had a critical issue where it would restart all node pools simultaneously during configuration changes. This violates OpenSearch cluster quorum requirements and causes cluster outages.

### Issue Details

- **Issue #650**: [#650](https://github.com/opensearch-project/opensearch-k8s-operator/issues/650) - Master nodes restart simultaneously across availability zones
- **Issue #738**: [#738](https://github.com/opensearch-project/opensearch-k8s-operator/issues/738) - All data node pools restart at the same time
- **Root Cause**: The operator treated each node pool independently during rolling restarts without considering cluster-wide role distribution, quorum requirements, or proper sequencing
- **Impact**: Production cluster outages when nodes are configured across availability zones, causing cluster status to turn red during updates

### Example Problematic Configuration

```yaml
nodePools:
  - component: master-a
    replicas: 1
    roles: ["cluster_manager"]
  - component: master-b  
    replicas: 1
    roles: ["cluster_manager"]
  - component: master-c
    replicas: 1
    roles: ["cluster_manager"]
  - component: data-b
    replicas: 2
    roles: ["data"]
  - component: data-c
    replicas: 2
    roles: ["data"]
```

## Solution Overview

Implemented a **comprehensive global candidate rolling restart strategy** that:

1. **Collects candidates across all StatefulSets** - Builds a global list of pods needing updates across all node types
2. **Applies intelligent candidate selection** - Prioritizes data nodes over master nodes, then sorts by StatefulSet name and highest ordinal
3. **Enforces master quorum preservation** - Ensures at least 2/3 masters are ready before restarting any master
4. **Restarts one pod at a time** - Only deletes one pod per reconciliation loop to maintain precise control

## Implementation Details

### Global Candidate Collection

The new `globalCandidateRollingRestart()` function:

1. **Iterates through all node pools** to find StatefulSets with pending updates
2. **Identifies pods needing updates** by comparing `UpdateRevision` with pod labels
3. **Builds a global candidate list** across all StatefulSets and availability zones

### Intelligent Candidate Selection

Candidates are sorted using a proven algorithm that ensures optimal restart order:

```go
// 1. Prioritize data nodes over master nodes for cluster stability
if !candidate.isMaster {
    // Data nodes get higher priority to minimize cluster impact
}

// 2. Sort by StatefulSet name (deterministic ordering across AZs)
sort.Slice(candidates, func(i, j int) bool {
    return candidates[i].sts.Name < candidates[j].sts.Name
})

// 3. Within each StatefulSet, prefer highest ordinal (drain from top)
sort.Slice(candidates, func(i, j int) bool {
    return candidates[i].ordinal > candidates[j].ordinal
})
```

### Master Quorum Preservation

Before restarting any master node:

```go
// Calculate cluster-wide master quorum
totalMasters, readyMasters := r.calculateMasterQuorum()

// Require at least 2/3 masters to be ready
requiredMasters := (totalMasters + 1) / 2
if readyMasters <= requiredMasters {
    // Skip master restart to preserve quorum
    continue
}
```

### One-Pod-at-a-Time Restart

The operator now deletes only one pod per reconciliation loop:

```go
// Find the best candidate
for _, candidate := range sortedCandidates {
    if r.isCandidateEligible(candidate) {
        return r.restartSpecificPod(candidate)
    }
}
```

### Key Functions

#### `globalCandidateRollingRestart()`
Main orchestrator function that:
- Collects all pods with pending updates across all StatefulSets and node types
- Applies intelligent candidate selection and sorting
- Enforces master quorum preservation
- Restarts one pod at a time per reconciliation loop

#### `restartSpecificPod()`
Handles the actual pod restart for a specific candidate:
- Performs the same prechecks as the original restart logic
- Deletes the specific pod to trigger StatefulSet rolling update
- Returns appropriate reconciliation result

#### `countMasters()`
Calculates cluster-wide master node quorum:
- Counts total master nodes across all master-eligible node pools
- Counts ready master nodes across all master-eligible node pools
- Used for quorum preservation decisions

#### `groupNodePoolsByRole()`
Groups node pools by role for analysis and logging:
- `dataOnly`: Node pools with only data role
- `dataAndMaster`: Node pools with both data and master roles  
- `masterOnly`: Node pools with only master role
- `other`: Node pools with other roles (ingest, etc.)

## Benefits

### 1. **Comprehensive Cluster Stability**
- Prevents simultaneous restart of all master nodes
- Ensures data nodes restart before master nodes for optimal cluster health
- Maintains OpenSearch cluster quorum requirements
- Eliminates production outages during configuration changes

### 2. **Multi-AZ and Multi-Node-Type Support**
- Properly handles all node types (master, data, coordinating, ingest) across multiple AZs
- Works with node provisioners like Karpenter that create separate node pools per AZ
- Ensures consistent restart behavior regardless of node distribution

### 3. **Predictable and Controlled Behavior**
- Clear restart order: data nodes → coordinating nodes → master nodes
- One pod restart at a time for precise control
- Maintains cluster health and availability during updates

### 4. **Backward Compatibility**
- No changes to existing API or configuration
- Works with existing cluster configurations

## Testing

### Unit Tests
- `TestGroupNodePoolsByRole()` - Validates role-based grouping logic used for analysis and logging
- `TestHasManagerRole()` / `TestHasDataRole()` - Validates role detection helper functions used throughout the implementation

### Integration Testing
The implementation includes comprehensive test scenarios in `test-scenario-rolling-restart.md`:

1. **Intelligent Candidate Selection** - Verifies data nodes restart before masters
2. **Master Quorum Protection** - Ensures restart is blocked when < 2/3 masters ready
3. **Multi-AZ Distribution** - Tests rolling restart across multiple availability zones
4. **One-Pod-at-a-Time** - Confirms only one pod restarts per reconciliation loop
5. **All Node Types** - Validates proper restart order for data, coordinating, and master nodes

### Test Cluster Configuration
A multi-AZ test cluster is provided in `test-multi-az-cluster.yaml` with:
- 3 master node pools across different AZs
- 3 data node pools across different AZs  
- 1 coordinating node pool
- Proper node selectors and tolerations for AZ distribution

## Migration Guide

### For Existing Clusters
No changes required. The new logic is automatically applied to existing clusters.

### For New Clusters
Continue using the same configuration format. The operator will automatically apply role-aware rolling restarts.

### Configuration Best Practices

1. **Master Node Distribution**
   ```yaml
   # Recommended: Distribute masters across AZs
   nodePools:
     - component: master-az1
       replicas: 1
       roles: ["cluster_manager"]
     - component: master-az2  
       replicas: 1
       roles: ["cluster_manager"]
     - component: master-az3
       replicas: 1
       roles: ["cluster_manager"]
   ```

2. **Data Node Configuration**
   ```yaml
   # Data nodes can be in separate pools
   nodePools:
     - component: data-hot
       replicas: 3
       roles: ["data", "data_hot"]
     - component: data-warm
       replicas: 2
       roles: ["data", "data_warm"]
   ```

## Monitoring and Observability

### Events
The operator now emits more detailed events during rolling restarts:
- `"Starting rolling restart"` - When restart begins
- `"Starting rolling restart of master node pool X"` - Master-specific restarts
- `"Skipping restart of master node pool X: insufficient quorum"` - Quorum preservation

### Logs
Enhanced logging provides visibility into:
- Role-based grouping decisions
- Quorum calculations
- Restart priority decisions
- Cluster health checks

## Future Enhancements

### Potential Improvements
1. **Configurable Restart Policies** - Allow users to customize restart behavior
2. **Health Check Integration** - Use OpenSearch health API for more sophisticated decisions
3. **Rollback Capabilities** - Automatic rollback if restart causes issues
4. **Metrics Integration** - Expose restart metrics for monitoring

### Configuration Options
Future versions could support:
```yaml
spec:
  rollingRestart:
    policy: "role-aware"  # or "legacy"
    masterQuorumThreshold: 0.5  # Custom quorum threshold
    maxConcurrentRestarts: 1    # Limit concurrent restarts
```

## Conclusion

This implementation resolves the critical issue of simultaneous master node restarts while maintaining backward compatibility and improving overall cluster stability. The role-aware approach ensures that OpenSearch clusters remain available during configuration changes, especially in multi-availability zone deployments.
