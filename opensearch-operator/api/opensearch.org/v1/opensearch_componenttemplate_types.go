package v1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpenSearchComponentTemplateState string

const (
	OpenSearchComponentTemplatePending OpenSearchComponentTemplateState = "PENDING"
	OpenSearchComponentTemplateCreated OpenSearchComponentTemplateState = "CREATED"
	OpenSearchComponentTemplateError   OpenSearchComponentTemplateState = "ERROR"
	OpenSearchComponentTemplateIgnored OpenSearchComponentTemplateState = "IGNORED"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchcomponenttemplate
//+kubebuilder:subresource:status

// OpenSearchComponentTemplate is the schema for the OpenSearch component templates API
type OpenSearchComponentTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchComponentTemplateSpec   `json:"spec,omitempty"`
	Status OpenSearchComponentTemplateStatus `json:"status,omitempty"`
}

type OpenSearchComponentTemplateStatus struct {
	State                     OpenSearchComponentTemplateState `json:"state,omitempty"`
	Reason                    string                           `json:"reason,omitempty"`
	ExistingComponentTemplate *bool                            `json:"existingComponentTemplate,omitempty"`
	ManagedCluster            *types.UID                       `json:"managedCluster,omitempty"`
	// Name of the currently managed component template
	ComponentTemplateName string `json:"componentTemplateName,omitempty"`
}

type OpenSearchComponentTemplateSpec struct {
	OpenSearchRef corev1.LocalObjectReference `json:"opensearchCluster"`

	// The name of the component template. Defaults to metadata.name
	// +immutable
	Name string `json:"name,omitempty"`

	// The template that should be applied
	Template OpenSearchIndexSpec `json:"template"`

	// Version number used to manage the component template externally
	Version int `json:"version,omitempty"`

	// If true, then indices can be automatically created using this template
	AllowAutoCreate bool `json:"allowAutoCreate,omitempty"`

	// Optional user metadata about the component template
	Meta *apiextensionsv1.JSON `json:"_meta,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchComponentTemplateList contains a list of OpenSearchComponentTemplate
type OpenSearchComponentTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchComponentTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenSearchComponentTemplate{}, &OpenSearchComponentTemplateList{})
}
