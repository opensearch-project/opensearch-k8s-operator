package v1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpenSearchIndexTemplateState string

const (
	OpenSearchIndexTemplatePending OpenSearchIndexTemplateState = "PENDING"
	OpenSearchIndexTemplateCreated OpenSearchIndexTemplateState = "CREATED"
	OpenSearchIndexTemplateError   OpenSearchIndexTemplateState = "ERROR"
	OpenSearchIndexTemplateIgnored OpenSearchIndexTemplateState = "IGNORED"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchindextemplate
//+kubebuilder:subresource:status

// OpenSearchIndexTemplate is the schema for the OpenSearch index templates API
type OpenSearchIndexTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchIndexTemplateSpec   `json:"spec,omitempty"`
	Status OpenSearchIndexTemplateStatus `json:"status,omitempty"`
}

type OpenSearchIndexTemplateStatus struct {
	State                 OpenSearchIndexTemplateState `json:"state,omitempty"`
	Reason                string                       `json:"reason,omitempty"`
	ExistingIndexTemplate *bool                        `json:"existingIndexTemplate,omitempty"`
	ManagedCluster        *types.UID                   `json:"managedCluster,omitempty"`
	// Name of the currently managed index template
	IndexTemplateName string `json:"indexTemplateName,omitempty"`
}

type OpenSearchIndexTemplateSpec struct {
	OpenSearchRef corev1.LocalObjectReference `json:"opensearchCluster"`

	// The name of the index template. Defaults to metadata.name
	// +immutable
	Name string `json:"name,omitempty"`

	// Array of wildcard expressions used to match the names of indices during creation
	IndexPatterns []string `json:"indexPatterns"`

	// The dataStream config that should be applied
	DataStream *OpenSearchDatastreamSpec `json:"dataStream,omitempty"`

	// The template that should be applied
	Template OpenSearchIndexSpec `json:"template,omitempty"`

	// An ordered list of component template names. Component templates are merged in the order specified,
	// meaning that the last component template specified has the highest precedence
	ComposedOf []string `json:"composedOf,omitempty"`

	// Priority to determine index template precedence when a new data stream or index is created.
	// The index template with the highest priority is chosen
	Priority int `json:"priority,omitempty"`

	// Version number used to manage the component template externally
	Version int `json:"version,omitempty"`

	// Optional user metadata about the index template
	Meta *apiextensionsv1.JSON `json:"_meta,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchIndexTemplateList contains a list of OpenSearchIndexTemplate
type OpenSearchIndexTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchIndexTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenSearchIndexTemplate{}, &OpenSearchIndexTemplateList{})
}
