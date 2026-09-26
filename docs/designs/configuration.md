# Cluster configuration

The configuration reconciler renders the OpenSearch settings into `opensearch.yml` and mounts it into the pods from a ConfigMap (subPath `config/opensearch.yml` under the OpenSearch home). Settings are not passed as environment variables.

The rendered file contains the settings added by the operator itself (TLS, security plugin defaults, gRPC, `node.attr.*` from `general.nodeAttributes`) merged with the user-supplied `additionalConfig`:

* `<cluster>-config`: the shared ConfigMap with the operator settings plus `general.additionalConfig`. It is also used by the bootstrap pod and the securityconfig update job.
* `<cluster>-<component>-config`: created only for node pools that set `nodePools[].additionalConfig`. It contains the shared settings with the pool's `additionalConfig` merged on top (pool values win). Those pools mount this ConfigMap instead of the shared one.

## Restarts on configuration changes

For every node pool the reconciler calculates a SHA1 hash over the pool's rendered `opensearch.yml`, the contents of ConfigMap/Secret `general.additionalVolumes` that set `restartPods: true` and, when TLS certificate hot reload is not enabled, the renewal markers of TLS certificates. The hash is written to the pod template as the `opensearch.org/config` annotation, so any change creates a new StatefulSet revision.

All node pool StatefulSets use the `OnDelete` update strategy, so Kubernetes never replaces pods by itself. The [rolling restart reconciler](rolling-restart-improvements.md) restarts every pod that is not on the latest revision, one pod at a time, across all node pools (data, coordinating and master-eligible alike).

## Configuration changes during upgrades

The hash is calculated for all node pools during a version upgrade too, so the StatefulSet template already carries the new configuration. The rolling restart reconciler does nothing while an upgrade is in progress (`status.version` differs from `spec.general.version`). Pods deleted by the [upgrade reconciler](upgrade.md) come back with both the new version and the new configuration. Once the upgrade is finished, the rolling restart reconciler restarts any pods that are still on an older revision.
