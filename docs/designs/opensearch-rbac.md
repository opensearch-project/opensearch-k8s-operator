# Opensearch Roles, Users, and Role Mappings reconciliation
Opensearch Roles, Users, and Role Mappings can be managed in a Kubernetes Native fashion by reconciling custom resources against the Opensearch API.  The resources are Cluster scoped, as Kubernetes namespacing doesn't make much sense when it's not reconciling other things in the k8s cluster.  Each resource needs to contain a reference to an OpenSearchCluster resource to reconcile against.

The reconciliation loop will need mechanisms to prevent overwriting or deleting Opensearch API objects that are not managed by k8s.  The Opensearch cluster reference should also not be changed as this can lead to orphaned objects or unexpected behaviour.  This is enforced by adding the Opensearch cluster reference to the status field when the object is first reconciled.  On subsequent operations if this does not match the reconciler will raise an error.

## Users
The name of the k8s OpensearchUser object will be the name of the User in Opensearch.  The majority of the CRD matches the Opensearch API, however the password must be stored in a secret, and a reference is passed to the custom resource.

When the user is created in the Opensearch API the k8s object UID is added as an attribute.  On all subsequent CRUD operations the UID is checked, and if it is not present, or does not match, then the operation will not be completed.

## Roles
The name of the k8s OpensearchRole object will be the name of the Role in Opensearch.  The OpensearchRoleSpec matches the Opensearch Roles API.

When an OpensearchRole is first reconciled the API is checked to see if the Role already exists.  If it does it is marked in the resource status, and no CRUD operations will be performed against the Opensearch API.


## Role Mappings
The operator uses OpensearchUserRoleBinding object that links users and roles together in a many <-> many relationship.  For each role the custom resource the operator will make sure there is a matching Role Mapping in the Opensearch API, that contains all of the users that are in the resource.

Due to the many <-> many nature of the binding, and the simplicity of Role Mappings there are not the same protections against CRUD operations.