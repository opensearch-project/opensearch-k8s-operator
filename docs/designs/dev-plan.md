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
8. Monitoring service, build-in prometheus exporter - In progress
9. Rolling upgrade - In progress
10. Cluster resources reconciler (Disk, CPU and Memory) - TODO
11. Cluster configuration reconciler (for opensearch.yaml configs) - TODO
12. Release automations and process (ECR, operatorshub, github) - In progress
13. Rolling restarts - for user requests - TODO.
14. Documentation (internal design and user guides) - In progress

# Phase 2
1. OpenSearch Operator CLI
2. Advanced shards allocation strategies
3. Automatic scaler
4. Snapshot and restore
