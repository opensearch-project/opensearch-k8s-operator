package requests

import apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

type IndexTemplate struct {
	IndexPatterns []string              `json:"index_patterns"`
	DataStream    *Datastream           `json:"data_stream,omitempty"`
	Template      Index                 `json:"template,omitempty"`
	ComposedOf    []string              `json:"composed_of,omitempty"`
	Priority      int                   `json:"priority,omitempty"`
	Version       int                   `json:"version,omitempty"`
	Meta          *apiextensionsv1.JSON `json:"_meta,omitempty"`
}

type ComponentTemplate struct {
	Template Index                 `json:"template"`
	Version  int                   `json:"version,omitempty"`
	Meta     *apiextensionsv1.JSON `json:"_meta,omitempty"`
}

type Index struct {
	Settings *apiextensionsv1.JSON `json:"settings,omitempty"`
	Mappings *apiextensionsv1.JSON `json:"mappings,omitempty"`
	Aliases  map[string]IndexAlias `json:"aliases,omitempty"`
}

type DatastreamTimestampFieldSpec struct {
	Name string `json:"name"`
}

type Datastream struct {
	TimestampField *DatastreamTimestampFieldSpec `json:"timestamp_field,omitempty"`
}

type IndexAlias struct {
	Index        string                `json:"index,omitempty"`
	Alias        string                `json:"alias,omitempty"`
	Filter       *apiextensionsv1.JSON `json:"filter,omitempty"`
	Routing      string                `json:"routing,omitempty"`
	IsWriteIndex bool                  `json:"is_write_index,omitempty"`
}
