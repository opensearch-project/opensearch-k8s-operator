package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpensearchTenantState string

const (
	OpensearchTenantPending OpensearchTenantState = "PENDING"
	OpensearchTenantCreated OpensearchTenantState = "CREATED"
	OpensearchTenantError   OpensearchTenantState = "ERROR"
	OpensearchTenantIgnored OpensearchTenantState = "IGNORED"
)

// OpensearchTenantSpec defines the desired state of OpensearchTenant
type OpensearchTenantSpec struct {
	OpensearchRef corev1.LocalObjectReference `json:"opensearchCluster"`
	Description   string                      `json:"description,omitempty"`
}

// OpensearchTenantStatus defines the observed state of OpensearchTenant
type OpensearchTenantStatus struct {
	State          OpensearchTenantState `json:"state,omitempty"`
	Reason         string                `json:"reason,omitempty"`
	ExistingTenant *bool                 `json:"existingTenant,omitempty"`
	ManagedCluster *types.UID            `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchtenant
//+kubebuilder:subresource:status

// OpensearchTenant is the Schema for the opensearchtenants API
type OpensearchTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchTenantSpec   `json:"spec,omitempty"`
	Status OpensearchTenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchTenantList contains a list of OpensearchTenant
type OpensearchTenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchTenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchTenant{}, &OpensearchTenantList{})
}
