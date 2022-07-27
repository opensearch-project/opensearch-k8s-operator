package requests

type RoleMapping struct {
	BackendRoles []string `json:"backend_roles,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	Users        []string `json:"users,omitempty"`
}
