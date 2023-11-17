package responses

import "github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"

type GetRoleMappingReponse map[string]requests.RoleMapping

type GetRoleResponse map[string]requests.Role

type GetUserResponse map[string]requests.User

type GetActionGroupResponse map[string]requests.ActionGroup

type GetTenantResponse map[string]requests.Tenant
