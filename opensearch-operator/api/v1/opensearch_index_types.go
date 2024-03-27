package v1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type OpensearchDatastreamTimestampFieldSpec struct {
	// Name of the field that are used for the DataStream
	Name string `json:"name"`
}

type OpensearchDatastreamSpec struct {
	// TimestampField for dataStream
	TimestampField OpensearchDatastreamTimestampFieldSpec `json:"timestamp_field,omitempty"`
}

// Describes the specs of an index
type OpensearchIndexSpec struct {
	// Configuration options for the index
	Settings *apiextensionsv1.JSON `json:"settings,omitempty"`

	// Mapping for fields in the index
	Mappings *apiextensionsv1.JSON `json:"mappings,omitempty"`

	// Aliases to add
	Aliases map[string]OpensearchIndexAliasSpec `json:"aliases,omitempty"`
}

// Describes the specs of an index alias
type OpensearchIndexAliasSpec struct {
	// The name of the index that the alias points to.
	Index string `json:"index,omitempty"`

	// The name of the alias.
	Alias string `json:"alias,omitempty"`

	// Query used to limit documents the alias can access.
	Filter *apiextensionsv1.JSON `json:"filter,omitempty"`

	// Value used to route indexing and search operations to a specific shard.
	Routing string `json:"routing,omitempty"`

	// If true, the index is the write index for the alias
	IsWriteIndex bool `json:"isWriteIndex,omitempty"`
}
