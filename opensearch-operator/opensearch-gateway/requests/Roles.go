package requests

type Role struct {
	ClusterPermissions []string                `json:"cluster_permissions,omitempty"`
	IndexPermissions   []IndexPermissionSpec   `json:"index_permissions,omitempty"`
	TenantPermissions  []TenantPermissionsSpec `json:"tenant_permissions,omitempty"`
}

type IndexPermissionSpec struct {
	IndexPatterns         []string `json:"index_patterns,omitempty"`
	DocumentLevelSecurity string   `json:"dls,omitempty"`
	FieldLevelSecurity    []string `json:"fls,omitempty"`
	AllowedActions        []string `json:"allowed_actions,omitempty"`
	MaskedFields          []string `json:"masked_fields,omitempty"`
}

type TenantPermissionsSpec struct {
	TenantPatterns []string `json:"tenant_patterns,omitempty"`
	AllowedActions []string `json:"allowed_actions,omitempty"`
}
