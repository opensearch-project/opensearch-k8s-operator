# Rolling Restart Design

## Overview

The rolling restart reconciler (`pkg/reconcilers/rollingRestart.go`) restarts pods whose StatefulSet has a pending revision, for example after a configuration change (see [configuration](configuration.md)). All node pool StatefulSets use the `OnDelete` update strategy, so this reconciler (or the [upgrade reconciler](upgrade.md) during version upgrades) is what replaces pods. It restarts one pod at a time across all node pools, keeps master quorum, and checks cluster health before each restart. The same logic applies to single-AZ and multi-AZ layouts (for example one node pool per zone).

## Preconditions

On each reconciliation the reconciler:

1. Does nothing while a version upgrade is in progress (`status.version` differs from `spec.general.version`).
2. Checks every node pool. If any StatefulSet has fewer ready pods than replicas, it requeues after 10 seconds and restarts nothing. Stuck pods on an older revision (for example in `CrashLoopBackOff` or `ImagePullBackOff`) are deleted to unblock the rollout, and Warning events are emitted.
3. Skips the restart until the cluster is initialized (`status.initialized`).
4. With `general.drainDataNodes`, removes stale shard allocation exclusions left by an earlier failed cleanup.

## Candidate selection

For every StatefulSet with `updatedReplicas` below `replicas`, the pod with an older revision becomes a candidate. The candidates are then sorted with a single comparator:

```go
// non-masters first, then StatefulSet name ascending, then ordinal descending
sort.Slice(candidates, func(i, j int) bool {
    if candidates[i].isMaster != candidates[j].isMaster {
        return !candidates[i].isMaster && candidates[j].isMaster
    }
    if candidates[i].sts.Name != candidates[j].sts.Name {
        return candidates[i].sts.Name < candidates[j].sts.Name
    }
    return candidates[i].ordinal > candidates[j].ordinal
})
```

So all non-master-eligible pods (data, coordinating, ingest, ...) are restarted first, ordered by StatefulSet name, then the master-eligible pods. A pool counts as master-eligible when it has the `master` or `cluster_manager` role, so a pool with both data and master roles is restarted with the masters.

## Master quorum

Before restarting a master-eligible pod the reconciler counts the replicas and ready pods across all master-eligible pools and requires:

```go
readyMasters > (totalMasters + 1) / 2   // integer division
```

With 3 masters all 3 must be ready. Since step 2 above already requires every pool to be fully ready, a master is in practice only restarted when all masters are ready. If the check fails, the reconciler falls back to a non-master candidate if there is one, and otherwise requeues after 10 seconds.

With a single master-eligible pod the check can never pass (`1 > 1` is false), so that pod is never restarted by this path.

## Restarting a pod

For the selected candidate the reconciler:

1. Checks cluster health with `CheckClusterStatusForRestart` and requeues after 10 seconds if the cluster is not ready. Green passes. Yellow passes only once shard allocation is enabled, all data nodes have joined and there is no shard activity (initializing, relocating, delayed unassigned shards or in-flight fetches). Red never passes.
2. Prepares the node with `PreparePodForDelete`: with `drainDataNodes` it excludes the node from shard allocation and waits for it to drain, otherwise it sets shard allocation to primaries only.
3. Deletes the pod. The StatefulSet recreates it with the new revision.
4. With `drainDataNodes`, removes the allocation exclusion. If that fails it requeues, and the stale exclusion is cleaned up on a later run.

Only one pod is deleted per reconciliation. The reconciler then requeues after 10 seconds and starts again from the preconditions, so the next pod is only restarted once the previous one is ready. When no pods are pending any more, shard allocation is re-enabled and the `RollingRestart` component status is set to `Finished`.

## Events

All events use the reason `RollingRestart`:

- Normal `Starting rolling restart`: emitted when a reconciliation finds pending pods and starts working on them.
- Normal `Rolling restart completed`: all pods are updated and shard allocation has been re-enabled.
- Warning `Deleted stuck pod '<pod>' in node pool '<pool>' to allow the rolling restart to proceed`.
- Warning `Pod '<pod>' in node pool '<pool>' is in <reason>; rolling restart is stalled until the pod becomes ready`.

Quorum and candidate decisions are logged, not emitted as events.

## Configuration

There are no rolling-restart specific settings. The behavior is influenced by `general.drainDataNodes` (drain nodes before restart instead of restricting allocation to primaries).
