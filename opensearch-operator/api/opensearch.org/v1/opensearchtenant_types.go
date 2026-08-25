package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpenSearchTenantState string

const (
	OpenSearchTenantPending OpenSearchTenantState = "PENDING"
	OpenSearchTenantCreated OpenSearchTenantState = "CREATED"
	OpenSearchTenantError   OpenSearchTenantState = "ERROR"
	OpenSearchTenantIgnored OpenSearchTenantState = "IGNORED"
)

// OpenSearchTenantSpec defines the desired state of OpenSearchTenant
type OpenSearchTenantSpec struct {
	OpenSearchRef corev1.LocalObjectReference `json:"opensearchCluster"`
	Description   string                      `json:"description,omitempty"`
}

// OpenSearchTenantStatus defines the observed state of OpenSearchTenant
type OpenSearchTenantStatus struct {
	State          OpenSearchTenantState `json:"state,omitempty"`
	Reason         string                `json:"reason,omitempty"`
	ExistingTenant *bool                 `json:"existingTenant,omitempty"`
	ManagedCluster *types.UID            `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchtenant
//+kubebuilder:subresource:status

// OpenSearchTenant is the Schema for the opensearchtenants API
type OpenSearchTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchTenantSpec   `json:"spec,omitempty"`
	Status OpenSearchTenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchTenantList contains a list of OpenSearchTenant
type OpenSearchTenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchTenant `json:"items"`
}

// GetOpenSearchRef returns the OpenSearch cluster reference
func (t *OpenSearchTenant) GetOpenSearchRef() corev1.LocalObjectReference {
	return t.Spec.OpenSearchRef
}

func init() {
	SchemeBuilder.Register(&OpenSearchTenant{}, &OpenSearchTenantList{})
}
