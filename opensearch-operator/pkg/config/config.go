package config

import _ "embed"

// go:embed action_groups.yml
var actionGroups []byte

// go:embed audit.yml
var audit []byte

// go:embed config.yml
var config []byte

// go:embed internal_users.yml
var internalUsers []byte

// go:embed nodes_dn.yml
var nodesDN []byte

// go:embed roles_mapping.yml
var rolesMapping []byte

// go:embed roles.yml
var roles []byte

// go:embed tenants.yml
var tenants []byte

// go:embed whitelist.yml
var whitelist []byte

var DefaultSecurityConfig = map[string]*[]byte{
	"action_groups.yml":  &actionGroups,
	"audit.yml":          &audit,
	"config.yml":         &config,
	"internal_uesrs.yml": &internalUsers,
	"nodes_dn.yml":       &nodesDN,
	"roles_mapping.yml":  &rolesMapping,
	"roles.yml":          &roles,
	"tenants.yml":        &tenants,
	"whitelist.yml":      &whitelist,
}
