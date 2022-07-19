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

type OpensearchUserRoleBindingState string

const (
	OpensearchUserRoleBindingPending      OpensearchUserRoleBindingState = "PENDING"
	OpensearchUserRoleBindingStateCreated OpensearchUserRoleBindingState = "CREATED"
	OpensearchUserRoleBindingStateError   OpensearchUserRoleBindingState = "ERROR"
)

// OpensearchUserRoleBindingSpec defines the desired state of OpensearchUserRoleBinding
type OpensearchUserRoleBindingSpec struct {
	OpensearchRef OpensearchClusterSelector `json:"opensearch"`
	Roles         []string                  `json:"roles"`
	Users         []string                  `json:"users"`
}

// OpensearchUserRoleBindingStatus defines the observed state of OpensearchUserRoleBinding
type OpensearchUserRoleBindingStatus struct {
	State            OpensearchUserRoleBindingState `json:"state,omitempty"`
	Reason           string                         `json:"reason,omitempty"`
	ManagedCluster   *OpensearchClusterSelector     `json:"managedCluster,omitempty"`
	ProvisionedRoles []string                       `json:"provisionedRoles,omitempty"`
	ProvisionedUsers []string                       `json:"provisionedUsers,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:resource:shortName=opensearchuserrolebinding
//+kubebuilder:subresource:status

// OpensearchUserRoleBinding is the Schema for the opensearchuserrolebindings API
type OpensearchUserRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchUserRoleBindingSpec   `json:"spec,omitempty"`
	Status OpensearchUserRoleBindingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchUserRoleBindingList contains a list of OpensearchUserRoleBinding
type OpensearchUserRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchUserRoleBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchUserRoleBinding{}, &OpensearchUserRoleBindingList{})
}
