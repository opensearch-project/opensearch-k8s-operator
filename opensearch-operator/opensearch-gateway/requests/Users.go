package requests

type User struct {
	Password                string            `json:"password,omitempty"`
	OpendistroSecurityRoles []string          `json:"opendistro_security_roles,omitempty"`
	BackendRoles            []string          `json:"backend_roles,omitempty"`
	Attributes              map[string]string `json:"attributes,omitempty"`
}
