package v1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpensearchIndexTemplateState string

const (
	OpensearchIndexTemplatePending OpensearchIndexTemplateState = "PENDING"
	OpensearchIndexTemplateCreated OpensearchIndexTemplateState = "CREATED"
	OpensearchIndexTemplateError   OpensearchIndexTemplateState = "ERROR"
	OpensearchIndexTemplateIgnored OpensearchIndexTemplateState = "IGNORED"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchindextemplate
//+kubebuilder:subresource:status

// OpensearchIndexTemplate is the schema for the OpenSearch index templates API
type OpensearchIndexTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchIndexTemplateSpec   `json:"spec,omitempty"`
	Status OpensearchIndexTemplateStatus `json:"status,omitempty"`
}

type OpensearchIndexTemplateStatus struct {
	State                 OpensearchIndexTemplateState `json:"state,omitempty"`
	Reason                string                       `json:"reason,omitempty"`
	ExistingIndexTemplate *bool                        `json:"existingIndexTemplate,omitempty"`
	ManagedCluster        *types.UID                   `json:"managedCluster,omitempty"`
	// Name of the currently managed index template
	IndexTemplateName string `json:"indexTemplateName,omitempty"`
}

type OpensearchIndexTemplateSpec struct {
	OpensearchRef corev1.LocalObjectReference `json:"opensearchCluster"`

	// The name of the index template. Defaults to metadata.name
	// +immutable
	Name string `json:"name,omitempty"`

	// Array of wildcard expressions used to match the names of indices during creation
	IndexPatterns []string `json:"indexPatterns"`

	// The dataStream config that should be applied
	DataStream *OpensearchDatastreamSpec `json:"dataStream,omitempty"`

	// The template that should be applied
	Template OpensearchIndexSpec `json:"template,omitempty"`

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

// OpensearchIndexTemplateList contains a list of OpensearchIndexTemplate
type OpensearchIndexTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchIndexTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchIndexTemplate{}, &OpensearchIndexTemplateList{})
}
