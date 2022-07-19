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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OpensearchRoleState string

const (
	OpensearchRoleStatePending OpensearchRoleState = "PENDING"
	OpensearchRoleStateCreated OpensearchRoleState = "CREATED"
	OpensearchRoleStateError   OpensearchRoleState = "ERROR"
)

// OpensearchRoleSpec defines the desired state of OpensearchRole
type OpensearchRoleSpec struct {
	OpensearchRef      OpensearchClusterSelector `json:"opensearch"`
	ClusterPermissions []string                  `json:"clusterPermissions,omitempty"`
	IndexPermissions   []IndexPermissionSpec     `json:"indexPermissions,omitempty"`
	TenantPermissions  []TenantPermissionsSpec   `json:"tenantPermissions,omitempty"`
}

type IndexPermissionSpec struct {
	IndexPatterns         []string `json:"indexPatterns,omitempty"`
	DocumentLevelSecurity string   `json:"dls,omitempty"`
	FieldLevelSecurity    []string `json:"fls,omitempty"`
	AllowedActions        []string `json:"allowedActions,omitempty"`
}

type TenantPermissionsSpec struct {
	TenantPatterns []string `json:"tenantPatterns,omitempty"`
	AllowedActions []string `json:"allowedActions,omitempty"`
}

// OpensearchRoleStatus defines the observed state of OpensearchRole
type OpensearchRoleStatus struct {
	State          OpensearchRoleState        `json:"state,omitempty"`
	Reason         string                     `json:"reason,omitempty"`
	ExistingRole   *bool                      `json:"existingRole,omitempty"`
	ManagedCluster *OpensearchClusterSelector `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:resource:shortName=opensearchrole
//+kubebuilder:subresource:status

// OpensearchRole is the Schema for the opensearchroles API
type OpensearchRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchRoleSpec   `json:"spec,omitempty"`
	Status OpensearchRoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchRoleList contains a list of OpensearchRole
type OpensearchRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchRole `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchRole{}, &OpensearchRoleList{})
}
