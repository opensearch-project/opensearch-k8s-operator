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

type OpenSearchUserState string

const (
	OpenSearchUserStatePending OpenSearchUserState = "PENDING"
	OpenSearchUserStateCreated OpenSearchUserState = "CREATED"
	OpenSearchUserStateError   OpenSearchUserState = "ERROR"
)

// OpenSearchUserSpec defines the desired state of OpenSearchUser
type OpenSearchUserSpec struct {
	OpenSearchRef           corev1.LocalObjectReference `json:"opensearchCluster"`
	PasswordFrom            corev1.SecretKeySelector    `json:"passwordFrom"`
	OpendistroSecurityRoles []string                    `json:"opendistroSecurityRoles,omitempty"`
	BackendRoles            []string                    `json:"backendRoles,omitempty"`
	Attributes              map[string]string           `json:"attributes,omitempty"`
}

// OpenSearchUserStatus defines the observed state of OpenSearchUser
type OpenSearchUserStatus struct {
	State          OpenSearchUserState `json:"state,omitempty"`
	Reason         string              `json:"reason,omitempty"`
	ManagedCluster *types.UID          `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=opensearchuser
//+kubebuilder:subresource:status

// OpenSearchUser is the Schema for the opensearchusers API
type OpenSearchUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenSearchUserSpec   `json:"spec,omitempty"`
	Status OpenSearchUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenSearchUserList contains a list of OpenSearchUser
type OpenSearchUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchUser `json:"items"`
}

// GetOpenSearchRef returns the OpenSearch cluster reference
func (u *OpenSearchUser) GetOpenSearchRef() corev1.LocalObjectReference {
	return u.Spec.OpenSearchRef
}

func init() {
	SchemeBuilder.Register(&OpenSearchUser{}, &OpenSearchUserList{})
}
