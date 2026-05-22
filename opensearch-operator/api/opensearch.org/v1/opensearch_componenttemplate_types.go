package v1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpensearchComponentTemplateState string

const (
	OpensearchComponentTemplatePending OpensearchComponentTemplateState = "PENDING"
	OpensearchComponentTemplateCreated OpensearchComponentTemplateState = "CREATED"
	OpensearchComponentTemplateError   OpensearchComponentTemplateState = "ERROR"
	OpensearchComponentTemplateIgnored OpensearchComponentTemplateState = "IGNORED"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchcomponenttemplate
//+kubebuilder:subresource:status

// OpensearchComponentTemplate is the schema for the OpenSearch component templates API
type OpensearchComponentTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchComponentTemplateSpec   `json:"spec,omitempty"`
	Status OpensearchComponentTemplateStatus `json:"status,omitempty"`
}

type OpensearchComponentTemplateStatus struct {
	State                     OpensearchComponentTemplateState `json:"state,omitempty"`
	Reason                    string                           `json:"reason,omitempty"`
	ExistingComponentTemplate *bool                            `json:"existingComponentTemplate,omitempty"`
	ManagedCluster            *types.UID                       `json:"managedCluster,omitempty"`
	// Name of the currently managed component template
	ComponentTemplateName string `json:"componentTemplateName,omitempty"`
}

type OpensearchComponentTemplateSpec struct {
	OpensearchRef corev1.LocalObjectReference `json:"opensearchCluster"`

	// The name of the component template. Defaults to metadata.name
	// +immutable
	Name string `json:"name,omitempty"`

	// The template that should be applied
	Template OpensearchIndexSpec `json:"template"`

	// Version number used to manage the component template externally
	Version int `json:"version,omitempty"`

	// If true, then indices can be automatically created using this template
	AllowAutoCreate bool `json:"allowAutoCreate,omitempty"`

	// Optional user metadata about the component template
	Meta *apiextensionsv1.JSON `json:"_meta,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchComponentTemplateList contains a list of OpensearchComponentTemplate
type OpensearchComponentTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchComponentTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchComponentTemplate{}, &OpensearchComponentTemplateList{})
}
