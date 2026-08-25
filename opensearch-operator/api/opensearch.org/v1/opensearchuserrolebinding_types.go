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

type OpenSearchUserRoleBindingState string

const (
	OpenSearchUserRoleBindingPending      OpenSearchUserRoleBindingState = "PENDING"
	OpenSearchUserRoleBindingStateCreated OpenSearchUserRoleBindingState = "CREATED"
	OpenSearchUserRoleBindingStateError   OpenSearchUserRoleBindingState = "ERROR"
)

// OpenSearchUserRoleBindingSpec defines the desired state of OpenSearchUserRoleBinding
type OpenSearchUserRoleBindingSpec struct {
	OpenSearchRef corev1.LocalObjectReference `json:"opensearchCluster"`
	Roles         []string                    `json:"roles"`
	Users         []string                    `json:"users,omitempty"`
	BackendRoles  []string                    `json:"backendRoles,omitempty"`
}

// OpenSearchUserRoleBindingStatus defines the observed state of OpenSearchUserRoleBinding
type OpenSearchUserRoleBindingStatus struct {
	State                   OpenSearchUserRoleBindingState `json:"state,omitempty"`
	Reason                  string                         `json:"reason,omitempty"`
	ManagedCluster          *types.UID                     `json:"managedCluster,omitempty"`
	ProvisionedRoles        []string                       `json:"provisionedRoles,omitempty"`
	ProvisionedUsers        []string                       `json:"provisionedUsers,omitempty"`
	ProvisionedBackendRoles []string                       `json:"provisionedBackendRoles,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchuserrolebinding
//+kubebuilder:subresource:status

// OpenSearchUserRoleBinding is the Schema for the opensearchuserrolebindings API
type OpenSearchUserRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchUserRoleBindingSpec   `json:"spec,omitempty"`
	Status OpenSearchUserRoleBindingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchUserRoleBindingList contains a list of OpenSearchUserRoleBinding
type OpenSearchUserRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchUserRoleBinding `json:"items"`
}

// GetOpenSearchRef returns the OpenSearch cluster reference
func (urb *OpenSearchUserRoleBinding) GetOpenSearchRef() corev1.LocalObjectReference {
	return urb.Spec.OpenSearchRef
}

func init() {
	SchemeBuilder.Register(&OpenSearchUserRoleBinding{}, &OpenSearchUserRoleBindingList{})
}
