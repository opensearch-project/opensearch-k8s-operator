# Development plan
The development plan currently has 2 phases.

# Phase 1
This phase contains both infra and app logic developments that are mandatory for the Operator capabilities.
Here are the steps arranged by priority:
1. Basic Operator manager to spin up a single cluster - Done
2. Testing framework - Done
3. Scaler service - Done
4. Spin up OpenSearch dashboard, with monitor and test - Done
5. Infrastructure for adding new on demand and worker services - Done
6. Basic OpenSearch Gateway, used for communicating with the OpenSearch cluster - Done
7. Security service - Done
8. Cluster resources reconciler (CPU and Memory) - Done
9. Release automations and process (ECR, operatorshub, github) - Done
10. Rolling upgrade - Done
11. Initial Documentation (internal design and user guides) - Done
12. Rolling restarts - for user requests - Done
13. Disk reconciler - TODO
14. Cluster configuration reconciler (for opensearch.yaml configs) - TODO

# Next Phases
1. Monitoring service, build-in prometheus exporter 
2. Snapshot and restore
3. Roles and users
4. Templates configs
5. ISM configs
6. Automatic scaler
7. OpenSearch Operator CLI
8. Advanced shards allocation strategies
