package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpenSearchActionGroupState string

const (
	OpenSearchActionGroupPending OpenSearchActionGroupState = "PENDING"
	OpenSearchActionGroupCreated OpenSearchActionGroupState = "CREATED"
	OpenSearchActionGroupError   OpenSearchActionGroupState = "ERROR"
	OpenSearchActionGroupIgnored OpenSearchActionGroupState = "IGNORED"
)

// OpenSearchActionGroupSpec defines the desired state of OpenSearchActionGroup
type OpenSearchActionGroupSpec struct {
	OpenSearchRef  corev1.LocalObjectReference `json:"opensearchCluster"`
	AllowedActions []string                    `json:"allowedActions"`
	Type           string                      `json:"type,omitempty"`
	Description    string                      `json:"description,omitempty"`
}

// OpenSearchActionGroupStatus defines the observed state of OpenSearchActionGroup
type OpenSearchActionGroupStatus struct {
	State               OpenSearchActionGroupState `json:"state,omitempty"`
	Reason              string                     `json:"reason,omitempty"`
	ExistingActionGroup *bool                      `json:"existingActionGroup,omitempty"`
	ManagedCluster      *types.UID                 `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchactiongroup
//+kubebuilder:subresource:status

// OpenSearchActionGroup is the Schema for the opensearchactiongroups API
type OpenSearchActionGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchActionGroupSpec   `json:"spec,omitempty"`
	Status OpenSearchActionGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchActionGroupList contains a list of OpenSearchActionGroup
type OpenSearchActionGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchActionGroup `json:"items"`
}

// GetOpenSearchRef returns the OpenSearch cluster reference
func (ag *OpenSearchActionGroup) GetOpenSearchRef() corev1.LocalObjectReference {
	return ag.Spec.OpenSearchRef
}

func init() {
	SchemeBuilder.Register(&OpenSearchActionGroup{}, &OpenSearchActionGroupList{})
}
