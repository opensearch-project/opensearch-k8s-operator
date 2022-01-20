# K8s Operator
The k8s OpenSearch Operator is composed of multiple entities, all deployed on kubernetes, allowing for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters.

![alt text](assets/K8sOperator.png "Operator components diagram")

# The operator entities are divided into the following categories:
- OpenSearch Operator
- Long living services
- Operators (Controllers)
- On demand workers services

# The OpenSearch Operator responsibilities
1. Load and maintains the long running services according to the installation config. 
2. Listen to API requests for operations need to be done on both cluster/s and kubernetes.
3. Provide status from operations executed by it.  

# Long running services
1. Initiated by the operator manager and will remain up as long the operator is up. 
2. Responsible for executing its logic every scheduling threshold.
3. Update its status to be used by the operator manager.

# Operators (controllers)
1. Initiated by the operator manager according to the configuration.
2. Watch it's CRD type for changes.
3. Loading a worker service for executing app logic.
4. Update its status to be used by the operator manager.

# On demand worker service
1. Initiated by the operator (controller).
2. Executes its operation and update the status to be used by the operator (controller). 

Most interactions with the operator will be performed through the operator manager.
This can be done in 2 ways:
1. Updating its yaml files and CRD. 
2. Sending API request.

- On both cases the flow will be as follow:
1. The Operator manager will create/update the operator (controller) CRD.
2. The Operator (controller) is watching the CRD for changes; if a change was made to the CRD that requires actions to be taken by its worker, it will load a worker to run the app login. Once triggered, the on demand service will communicate with the OpenSearch clusters through the OpenSearch Gateway using OpenSearch REST API
3. On successful worker logic execution, the Operator (controller) will communicate with kubernetes through k8s APIs, modifying resources files as needed.