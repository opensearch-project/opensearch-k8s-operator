/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpenSearchRoleState string

const (
	OpenSearchRoleStatePending OpenSearchRoleState = "PENDING"
	OpenSearchRoleStateCreated OpenSearchRoleState = "CREATED"
	OpenSearchRoleStateError   OpenSearchRoleState = "ERROR"
	OpenSearchRoleIgnored      OpenSearchRoleState = "IGNORED"
)

// OpenSearchRoleSpec defines the desired state of OpenSearchRole
type OpenSearchRoleSpec struct {
	OpenSearchRef      corev1.LocalObjectReference `json:"opensearchCluster"`
	ClusterPermissions []string                    `json:"clusterPermissions,omitempty"`
	IndexPermissions   []IndexPermissionSpec       `json:"indexPermissions,omitempty"`
	TenantPermissions  []TenantPermissionsSpec     `json:"tenantPermissions,omitempty"`
}

type IndexPermissionSpec struct {
	IndexPatterns         []string `json:"indexPatterns,omitempty"`
	DocumentLevelSecurity string   `json:"dls,omitempty"`
	FieldLevelSecurity    []string `json:"fls,omitempty"`
	AllowedActions        []string `json:"allowedActions,omitempty"`
	MaskedFields          []string `json:"maskedFields,omitempty"`
}

type TenantPermissionsSpec struct {
	TenantPatterns []string `json:"tenantPatterns,omitempty"`
	AllowedActions []string `json:"allowedActions,omitempty"`
}

// OpenSearchRoleStatus defines the observed state of OpenSearchRole
type OpenSearchRoleStatus struct {
	State          OpenSearchRoleState `json:"state,omitempty"`
	Reason         string              `json:"reason,omitempty"`
	ExistingRole   *bool               `json:"existingRole,omitempty"`
	ManagedCluster *types.UID          `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchrole
//+kubebuilder:subresource:status

// OpenSearchRole is the Schema for the opensearchroles API
type OpenSearchRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchRoleSpec   `json:"spec,omitempty"`
	Status OpenSearchRoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchRoleList contains a list of OpenSearchRole
type OpenSearchRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchRole `json:"items"`
}

// GetOpenSearchRef returns the OpenSearch cluster reference
func (r *OpenSearchRole) GetOpenSearchRef() corev1.LocalObjectReference {
	return r.Spec.OpenSearchRef
}

func init() {
	SchemeBuilder.Register(&OpenSearchRole{}, &OpenSearchRoleList{})
}
