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
)

type OpensearchUserState string

const (
	OpensearchUserStatePending OpensearchUserState = "PENDING"
	OpensearchUserStateCreated OpensearchUserState = "CREATED"
	OpensearchUserStateError   OpensearchUserState = "ERROR"
)

// OpensearchUserSpec defines the desired state of OpensearchUser
type OpensearchUserSpec struct {
	OpensearchRef           OpensearchClusterSelector `json:"opensearch"`
	PasswordFrom            UserPasswordSpec          `json:"passwordFrom"`
	OpendistroSecurityRoles []string                  `json:"opendistroSecurityRoles,omitempty"`
	BackendRoles            []string                  `json:"backendRoles,omitempty"`
	Attributes              map[string]string         `json:"attributes,omitempty"`
}

type UserPasswordSpec struct {
	corev1.SecretKeySelector `json:",inline"`
	Namespace                string `json:"namespace,omitempty"`
}

// OpensearchUserStatus defines the observed state of OpensearchUser
type OpensearchUserStatus struct {
	State          OpensearchUserState        `json:"state,omitempty"`
	Reason         string                     `json:"reason,omitempty"`
	ManagedCluster *OpensearchClusterSelector `json:"managedCluster,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:resource:shortName=opensearchuser
//+kubebuilder:subresource:status

// OpensearchUser is the Schema for the opensearchusers API
type OpensearchUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchUserSpec   `json:"spec,omitempty"`
	Status OpensearchUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchUserList contains a list of OpensearchUser
type OpensearchUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchUser{}, &OpensearchUserList{})
}
