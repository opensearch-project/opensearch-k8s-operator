# Security Controller

The security controller deals with everything related to the opensearch-security plugin. This entails two areas:

* Configuring TLS for nodes
* Manging the securityconfig

The controller is configured via the `security` object in the cluster spec. If no config is provided the controller will fall back to the demo certificates and configuration that is included with the opensearch docker image.

## Features for Phase 1

* Generate self-signed certificates for the cluster to use
* Use existing certificates provided via secrets
* Allow user to supply custom securityconfig via ConfigMap

## Features for Phase 2

* Certificates per nodepool
* Manage securityconfig via extra CRDs (e.g. OpenSearchUser, OpenSearchRole, etc.)
