package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpensearchActionGroupState string

const (
	OpensearchActionGroupPending OpensearchActionGroupState = "PENDING"
	OpensearchActionGroupCreated OpensearchActionGroupState = "CREATED"
	OpensearchActionGroupError   OpensearchActionGroupState = "ERROR"
	OpensearchActionGroupIgnored OpensearchActionGroupState = "IGNORED"
)

// OpensearchActionGroupSpec defines the desired state of OpensearchActionGroup
type OpensearchActionGroupSpec struct {
	OpensearchRef  corev1.LocalObjectReference `json:"opensearchCluster"`
	AllowedActions []string                    `json:"allowedActions"`
	Type           string                      `json:"type,omitempty"`
	Description    string                      `json:"description,omitempty"`
}

// OpensearchActionGroupStatus defines the observed state of OpensearchActionGroup
type OpensearchActionGroupStatus struct {
	State               OpensearchActionGroupState `json:"state,omitempty"`
	Reason              string                     `json:"reason,omitempty"`
	ExistingActionGroup *bool                      `json:"existingActionGroup,omitempty"`
	ManagedCluster      *types.UID                 `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchactiongroup
//+kubebuilder:subresource:status

// OpensearchActionGroup is the Schema for the opensearchactiongroups API
type OpensearchActionGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchActionGroupSpec   `json:"spec,omitempty"`
	Status OpensearchActionGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchActionGroupList contains a list of OpensearchActionGroup
type OpensearchActionGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchActionGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchActionGroup{}, &OpensearchActionGroupList{})
}
