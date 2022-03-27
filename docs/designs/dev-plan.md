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
10. Rolling upgrade - In progress
11. Documentation (internal design and user guides) - In progress
12. Disk reconciler - TODO
13. Cluster configuration reconciler (for opensearch.yaml configs) - TODO
14. Rolling restarts - for user requests - TODO

# Next Phases
1. OpenSearch Operator CLI
2. Advanced shards allocation strategies
3. Automatic scaler
4. Snapshot and restore
5. Roles and users 
6. Monitoring service, build-in prometheus exporter 
7. Templates configs
8. ILM configs
