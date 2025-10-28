# Rolling Restart Design for Multi-AZ OpenSearch Clusters

## Overview

This document describes the design for rolling restart functionality in the OpenSearch Kubernetes operator. The design ensures cluster stability and availability during configuration updates by implementing a role-aware, quorum-preserving restart strategy that works across multiple availability zones and node types.

## Design Principles

The rolling restart implementation follows these core principles:

1. **Cluster Stability First** - Maintain OpenSearch cluster quorum and health during restarts
2. **Role-Aware Sequencing** - Restart nodes in an order that minimizes cluster impact
3. **Multi-AZ Support** - Handle node pools distributed across multiple availability zones
4. **One-at-a-Time Control** - Restart only one pod per reconciliation loop for precise control
5. **Backward Compatibility** - Work with existing cluster configurations without changes

## Architecture

### Global Candidate Rolling Restart Strategy

The operator implements a **comprehensive global candidate rolling restart strategy** that:

1. **Collects candidates across all StatefulSets** - Builds a global list of pods needing updates across all node types
2. **Applies intelligent candidate selection** - Prioritizes data nodes over master nodes, then sorts by StatefulSet name and highest ordinal
3. **Enforces master quorum preservation** - Ensures at least 2/3 masters are ready before restarting any master
4. **Restarts one pod at a time** - Only deletes one pod per reconciliation loop to maintain precise control

## Implementation Design

### Global Candidate Collection

The `globalCandidateRollingRestart()` function:

1. **Iterates through all node pools** to find StatefulSets with pending updates
2. **Identifies pods needing updates** by comparing `UpdateRevision` with pod labels
3. **Builds a global candidate list** across all StatefulSets and availability zones

### Intelligent Candidate Selection

Candidates are sorted using an algorithm that ensures optimal restart order:

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

The operator deletes only one pod per reconciliation loop:

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


## Design Benefits

### 1. **Comprehensive Cluster Stability**
- Prevents simultaneous restart of all master nodes
- Ensures data nodes restart before master nodes for optimal cluster health
- Maintains OpenSearch cluster quorum requirements
- Preserves cluster availability during configuration changes

### 2. **Multi-AZ and Multi-Node-Type Support**
- Handles all node types (master, data, coordinating, ingest) across multiple AZs
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
The design includes comprehensive test scenarios:

1. **Intelligent Candidate Selection** - Verifies data nodes restart before masters
2. **Master Quorum Protection** - Ensures restart is blocked when < 2/3 masters ready
3. **Multi-AZ Distribution** - Tests rolling restart across multiple availability zones
4. **One-Pod-at-a-Time** - Confirms only one pod restarts per reconciliation loop
5. **All Node Types** - Validates proper restart order for data, coordinating, and master nodes

### Test Cluster Configuration
A multi-AZ test cluster configuration includes:
- 3 master node pools across different AZs
- 3 data node pools across different AZs  
- 1 coordinating node pool
- Proper node selectors and tolerations for AZ distribution

## Usage

### For Existing Clusters
No changes required. The rolling restart logic is automatically applied to existing clusters.

### For New Clusters
Use the standard configuration format. The operator will automatically apply role-aware rolling restarts.

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
The operator emits detailed events during rolling restarts:
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

## Summary

This design provides a comprehensive rolling restart strategy that ensures cluster stability and availability during configuration updates. The role-aware approach maintains OpenSearch cluster quorum requirements and works effectively across multiple availability zones and node types, providing predictable and controlled restart behavior.
