# OpenSearch Cluster Status ConfigMap — SRE Reference

The operator automatically publishes a live diagnostics snapshot for each
OpenSearch cluster into a Kubernetes ConfigMap. This document explains what
is in the ConfigMap, how to read it, and the commands you need for day-to-day
triage and incident response.

---

## Overview

When the `statusConfigMap` feature is enabled on an `OpenSearchCluster` CR, the
operator creates and continuously refreshes a ConfigMap named
**`<cluster-name>-status`** in the same namespace as the cluster.

The ConfigMap contains two keys:

| Key | Contents |
|---|---|
| `diagnostics` | Full cluster snapshot — all sections, every reconcile cycle |
| `problems` | Only the sections where something is wrong. Returns `No problems detected.` on a healthy cluster |

The ConfigMap is intended to be **read-only**

---

## Enabling the Feature

Add (or patch) the `statusConfigMap` block in `spec.general` of the
`OpenSearchCluster` CR:

```yaml
spec:
  general:
    statusConfigMap:
      enable: true
      updateInterval: 60s   # how often the operator refreshes the ConfigMap
```

Or patch an existing cluster live:

```bash
kubectl patch opensearchcluster <cluster-name> -n <namespace> \
  --type=merge \
  -p '{"spec":{"general":{"statusConfigMap":{"enable":true,"updateInterval":"60s"}}}}'
```

Disabling the feature (`enable: false`) causes the operator to delete the
ConfigMap. The default `updateInterval` is 60 seconds.

---

## ConfigMap Name and Location

```
<cluster-name>-status   in the same namespace as the OpenSearchCluster
```

Example — cluster `my-cluster` in namespace `opensearch`:

```bash
kubectl get configmap my-cluster-status -n opensearch
```

---

## Getting the Output

### Full diagnostics report

```bash
kubectl get configmap <cluster-name>-status -n <namespace> \
  -o jsonpath='{.data.diagnostics}'
```

### Quick-check: is the cluster healthy?

```bash
kubectl get configmap <cluster-name>-status -n <namespace> \
  -o jsonpath='{.data.problems}'
```

---

## What the Diagnostics Key Shows

The `diagnostics` key always contains all sections regardless of cluster state.


### OVERVIEW

```
=== OVERVIEW ===
Cluster: <name>
Last Updated: 2026-07-08 17:55:00 UTC
```

The timestamp is UTC. Use it to judge data freshness before acting on the report.

---

### CLUSTER HEALTH

```
=== CLUSTER HEALTH ===
Status:               yellow
Unassigned Shards:    3       ← only shown when > 0
Busy Threads:         2       ← only shown when > 0
Pending Tasks:        1       ← only shown when > 0
Nodes Under Pressure: 1       ← only shown when > 0
```

`Status` maps directly to the OpenSearch cluster health: `green`, `yellow`, or
`red`. When status is not green, a **NON-GREEN INDICES** sub-table follows
immediately showing which indices are degraded and how many shards are missing.

---

### NON-GREEN INDICES

```
=== NON-GREEN INDICES ===
INDEX              HEALTH   UNASSIGNED.PRIMARY   UNASSIGNED.SHARDS
my-index           yellow   0                    2
critical-index     red      1                    3
```

Red indices (unavailable primaries) are listed before yellow. Up to 20 indices
are shown; a `... and N more` footer appears when the list is truncated.

---

### NODES

```
=== NODES ===
Nodes:       3
Data Nodes:  3

NAME                             IP          HEAP   RAM    CPU   LOAD   ROLE
my-cluster-masters-0 *           10.0.0.1    32%    61%    4%    1.20   dm
my-cluster-masters-1             10.0.0.2    28%    59%    3%    0.95   dm
my-cluster-masters-2             10.0.0.3    30%    60%    4%    1.10   dm
(* = elected cluster manager)
```

`*` marks the elected cluster manager node. Up to 50 nodes are shown.

Column meanings:
| Column | Meaning |
|---|---|
| `HEAP` | JVM heap utilisation % |
| `RAM` | OS RAM utilisation % |
| `CPU` | CPU utilisation % |
| `LOAD` | 1-minute load average |
| `ROLE` | OpenSearch node roles abbreviation (`d`=data, `m`=cluster_manager, `i`=ingest, etc.) |

---

### Cluster Manager

```
=== Cluster Manager ===
ID                   HOST       IP         NODE
abc123...            10.0.0.1   10.0.0.1   my-cluster-masters-0
```

---

### INDICES (Top 10 by size)

```
=== INDICES (Top 10 by size) ===
HEALTH   STATUS   INDEX        PRI   REP   DOCS        SIZE
green    open     logs-2026    3     1     4200000     12.3gb
yellow   open     old-index    1     1     10000       45.2mb
```

Sorted largest-first by on-disk size. The cap is 10 indices in the ConfigMap
view; the `diagnostics-dump` export shows all indices.

---

### SHARDS

```
=== SHARDS ===
Total Shards:        48
Primary Shards:      24
Replica Shards:      24
Active Shards:       45
Relocating Shards:   0
Initializing Shards: 0
Unassigned Shards:   3

=== UNASSIGNED SHARDS ===
[my-index] shard 0 · replica
  reason:      INDEX_CREATED
  explanation: cannot allocate because allocation is not permitted to any of the nodes
  blocked on:
    my-cluster-masters-1                same_shard
```

For each unassigned shard the operator inlines an `allocation/explain` result
(up to 5 shards in the ConfigMap view), showing:
- `state` and `can_allocate` from the OpenSearch allocator
- `reason` — why the shard became unassigned (e.g. `NODE_LEFT`, `ALLOCATION_FAILED`)
- `explanation` — human-readable summary
- Per-node deciders that blocked placement (YES decisions are omitted as noise)

Up to 20 unassigned shards are listed in the ConfigMap; a `... and N more` line
appears when the list is truncated.

---

### ACTIVE RECOVERY

```
=== ACTIVE RECOVERY ===
INDEX       SHARD   TYPE       STAGE   SOURCE                TARGET               FILES%   BYTES%   TIME
my-index    0       REPLICA    INDEX   my-cluster-masters-0  my-cluster-masters-1  72.0%    68.4%   00:01:32
```

Shows shards currently being recovered/relocated. Empty `(none)` when no
recoveries are in progress.

---

### ALLOCATION

```
=== ALLOCATION ===
SHARDS   DISK.INDICES   DISK.USED   DISK.AVAIL   DISK.TOTAL   DISK.PERCENT   HOST       IP         NODE
16       12.3gb         45gb        55gb         100gb        45%            10.0.0.1   10.0.0.1   my-cluster-masters-0
```

Disk usage per node. `DISK.PERCENT` is the percentage of total disk used. Up
to 50 rows are shown.

---

### THREAD POOLS (queue/rejected > 0)

```
=== THREAD POOLS (queue/rejected > 0) ===
NODE                     POOL     ACTIVE   QUEUE   REJECTED
my-cluster-masters-0     write    4        120     8500
my-cluster-masters-0     search   2        45      0
```

Only thread pools with a non-zero queue or rejection count are shown. Sorted
most-rejected first, then most-queued. Empty `(none)` on a healthy cluster.
Note: Thread pool rejections are cumulative, a non-zero `REJECTED` count does not
necessarily mean the pool is currently under pressure, but a growing count
across refreshes does.

---

### PENDING TASKS

```
=== PENDING TASKS ===
ORDER   TIME.IN.QUEUE   PRIORITY   SOURCE
42      5s              URGENT     put-mapping [my-index]
```

Cluster-manager tasks that are queued but not yet executed. A persistent backlog
here indicates the cluster manager is overloaded. Empty `(none)` is normal.

---

### CIRCUIT BREAKERS (>= 95% utilisation)

```
=== CIRCUIT BREAKERS (>= 95% utilisation) ===
NODE                     BREAKER          UTIL%   TRIPPED   LIMIT    ESTIMATED
my-cluster-masters-0     fielddata        97.3%   2         512.0m   498.1m
```

Only breakers with current estimated utilisation ≥ 95% of their limit are
shown. `TRIPPED` is a cumulative lifetime counter (does not reset). Empty
`(none)` is normal. Breakers nearing their limit indicate imminent OOM risk on
that node.

---

### SEGMENTS (Top 10 by size)

```
=== SEGMENTS (Top 10 by size) ===
INDEX       SHARD   PRIREP   IP         SEGMENT   GENERATION   DOCS.COUNT   DOCS.DELETED   SIZE      ...
my-index    0       p        10.0.0.1   _2a        42           150000       200            120.3mb   ...
```

Lucene segment details sorted largest-first. Useful for diagnosing segment
bloat or high deleted-doc ratios. Up to 10 segments shown.

---

## What the Problems Key Shows

The `problems` key is a **filtered subset** of `diagnostics`. It only renders
sections that indicate something is wrong:

| Problem check | Appears when |
|---|---|
| Cluster health | Status is not `green` |
| Non-green indices | Any index is `yellow` or `red` |
| Unassigned shards | Any shard is in `UNASSIGNED` state |
| Thread pool backlog | Any pool has queue > 0 or rejections > 0 |
| Pending tasks | Any cluster-manager tasks are queued |
| Circuit breakers | Any breaker is at ≥ 95% utilisation |

When every check is clear, `problems` contains exactly one line:

```
No problems detected.
```

## Full Uncapped Export (`diagnostics-dump`)

The ConfigMap is capped per section so it stays under the ~1 MiB Kubernetes
etcd object limit. If you want to see the full, uncapped report, run the 
`diagnostics-dump` subcommand via `kubectl exec` against the operator
pod to get an uncapped report written to stdout:

```bash
kubectl exec deploy/<operator-deployment> -n <operator-namespace> -- \
  /manager diagnostics-dump \
  --cluster <cluster-name> \
  --namespace <cluster-namespace> \
  > diagnostics-full.txt
```

Example:

```bash
kubectl exec deploy/opensearch-operator-controller-manager -n opensearch-operator-system -- \
  /manager diagnostics-dump \
  --cluster my-cluster \
  --namespace opensearch \
  > diagnostics-full-$(date +%Y%m%d-%H%M%S).txt
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--cluster` | *(required)* | Name of the `OpenSearchCluster` to report on |
| `--namespace` | *(required)* | Namespace of the `OpenSearchCluster` |
| `--max-explain` | `100` | Max unassigned shards to run `allocation/explain` on (one API call each; `<=0` means all — expensive on a badly broken cluster) |

Notes:
- Progress and error messages go to **stderr**; only the report goes to stdout,
  so the redirected file contains only the report text.
- No additional access is required. The subcommand runs as the operator's
  ServiceAccount, which already has access to the cluster credentials.
- If the operator pod runs more than one container, add
  `-c operator-controller-manager` to the `kubectl exec`.


---

## Troubleshooting

| Symptom | Likely cause | Action |
|---|---|---|
| ConfigMap not found | Feature not enabled, or cluster not yet `Running` | Check `kubectl get opensearchcluster <name> -n <ns>` — `statusConfigMap.enable` must be `true` and phase must be `Running` |
| ConfigMap exists but data is stale | Operator pod is down or erroring | Check `kubectl logs deploy/<operator> -n <operator-ns> \| grep -E "status_configmap\|error\|Error"` |
| `diagnostics` shows `ERROR: could not fetch any cluster data` | Operator cannot reach OpenSearch | Verify OpenSearch pods are up; check operator logs for connection errors |
| Individual section shows `ERROR: could not fetch <section>` | That specific API call failed | Often transient; check if it clears on the next refresh cycle |
| `problems` is empty string (not `No problems detected.`) | Feature disabled or ConfigMap just deleted | Re-enable via patch; wait one `updateInterval` |
| `diagnostics-dump` exits with `ERROR connecting to OpenSearch` | Credentials or network issue inside the pod | Run `kubectl exec` into the operator pod and verify connectivity to the OpenSearch service |
