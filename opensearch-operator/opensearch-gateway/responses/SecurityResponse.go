package responses

import "opensearch.opster.io/opensearch-gateway/requests"

type GetRoleMappingReponse map[string]requests.RoleMapping

type GetRoleResponse map[string]requests.Role

type GetUserResponse map[string]requests.User

type GetActionGroupResponse map[string]requests.ActionGroup

type GetTenantResponse map[string]requests.Tenant
