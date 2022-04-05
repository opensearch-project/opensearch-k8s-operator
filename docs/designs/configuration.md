# Cluster configuration

The operator makes use of two methods for configuring a cluster.  For user defined additional configuration key value pairs, these are added to the pods as environment variable.  On startup the Opensearch containers will load the environment variables as Opensearch configuration.  Security plugin configurations that are applied by the operator are added to the `opensearch.yml` config file which is then mounted into the container.

## Rolling restarts

If a config file is being injected into the pods then a SHA1 hash is calculated for the content of the file and added as an annotation to the pods.  This allows us to detect changes which will trigger restarts.  For environment variable changes these will also result in a restart.

For non data nodes the Kubernetes stateful set controller will restart the pods.  For data nodes the rolling restart reconciler will detect if there is a pending change to the pods and gracefully restart them.

## Configuration changes during upgrades

When a rolling upgrade is in flight non data nodes will not be modified to prevent unexpected restarts.  These changes will be picked up after the data nodes have been upgraded.  For data nodes the changes will be added.  These will be picked up during the restarts for the rolling upgrade.  If a data node pool has already been restarted they will be picked up by the restart reconciler.

To achieve this the operator tracks the changes per node pool, and decides whether to update the hash and environment variables independently for each node pool.