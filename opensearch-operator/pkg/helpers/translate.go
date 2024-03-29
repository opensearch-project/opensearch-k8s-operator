package helpers

import (
	v1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
)

// TranslateIndexTemplateToRequest rewrites the CRD format to the gateway format
func TranslateIndexTemplateToRequest(spec v1.OpensearchIndexTemplateSpec) requests.IndexTemplate {
	request := requests.IndexTemplate{
		IndexPatterns: spec.IndexPatterns,
		Template:      TranslateIndexToRequest(spec.Template),
		Priority:      spec.Priority,
		Version:       spec.Version,
	}
	if spec.Meta.Size() > 0 {
		request.Meta = spec.Meta
	}
	if len(spec.ComposedOf) > 0 {
		request.ComposedOf = spec.ComposedOf
	}

	return request
}

// TranslateComponentTemplateToRequest rewrites the CRD format to the gateway format
func TranslateComponentTemplateToRequest(spec v1.OpensearchComponentTemplateSpec) requests.ComponentTemplate {
	request := requests.ComponentTemplate{
		Template: TranslateIndexToRequest(spec.Template),
		Version:  spec.Version,
	}
	if spec.Meta.Size() > 0 {
		request.Meta = spec.Meta
	}

	return request
}

// TranslateIndexToRequest rewrites the CRD format to the gateway format
func TranslateIndexToRequest(spec v1.OpensearchIndexSpec) requests.Index {
	aliases := make(map[string]requests.IndexAlias)
	for key, val := range spec.Aliases {
		aliases[key] = requests.IndexAlias{
			Index:        val.Index,
			Alias:        val.Alias,
			Filter:       val.Filter,
			Routing:      val.Routing,
			IsWriteIndex: val.IsWriteIndex,
		}
	}

	request := requests.Index{}

	if len(aliases) > 0 {
		request.Aliases = aliases
	}
	if spec.Settings.Size() > 0 {
		request.Settings = spec.Settings
	}
	if spec.Mappings.Size() > 0 {
		request.Mappings = spec.Mappings
	}

	return request
}
