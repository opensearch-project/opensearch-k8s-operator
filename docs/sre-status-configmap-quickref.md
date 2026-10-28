# Status ConfigMap — Quick Reference

The operator publishes a live diagnostics snapshot into a read-only ConfigMap
named **`<cluster-name>-status`** (same namespace as the cluster).

**Keys:** \
`diagnostics` (full snapshot, all sections) \
`problems` (Use this for a quick check of only broken sections; `No problems detected.` when healthy)

## Enable

```yaml
spec:
  general:
    statusConfigMap:
      enable: true
      updateInterval: 60s   # default 60s; enable:false deletes the ConfigMap
```

## Read

```bash
# Full report
kubectl get cm <cluster-name>-status -n <ns> -o jsonpath='{.data.diagnostics}'
# Health-only check
kubectl get cm <cluster-name>-status -n <ns> -o jsonpath='{.data.problems}'
```

## Sections (in `diagnostics`)

| Section | Shows | Cap |
|---|---|---|
| `OVERVIEW` | Cluster name + UTC last-updated timestamp (freshness) | — |
| `CLUSTER HEALTH` | `green`/`yellow`/`red`; unassigned shards, busy threads, pending tasks, nodes under pressure (only if > 0) | — |
| `NON-GREEN INDICES` | Degraded indices + unassigned primaries/shards (red first) | 20 |
| `NODES` | Per-node HEAP/RAM/CPU/LOAD/ROLE; `*` = elected manager | 50 |
| `Cluster Manager` | Current elected manager node | — |
| `INDICES` | Top indices by on-disk size (health, docs, size) | 10 |
| `SHARDS` | Totals + per-shard `allocation/explain` (reason, blocking deciders) | 20 (5 explained) |
| `ACTIVE RECOVERY` | In-progress recoveries/relocations; `(none)` if idle | — |
| `ALLOCATION` | Per-node disk usage (`DISK.PERCENT` = % used) | 50 |
| `THREAD POOLS` | Pools with queue/rejected > 0 (rejections cumulative) | — |
| `PENDING TASKS` | Queued cluster-manager tasks; backlog = overloaded manager | — |
| `CIRCUIT BREAKERS` | Breakers ≥ 95% util (OOM risk); `TRIPPED` is lifetime | — |
| `SEGMENTS` | Top Lucene segments by size (bloat / deleted-doc ratio) | 10 |

## Full uncapped export

```bash
kubectl exec deploy/<operator> -n <operator-ns> -- \
  /manager diagnostics-dump --cluster <name> --namespace <ns> > full.txt
```

For a full guide, refer here: [SRE Status ConfigMap Reference](./sre-status-configmap.md)