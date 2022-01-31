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

const (
	PhasePending = "PENDING"
	PhaseRunning = "RUNNING"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type GeneralConfig struct {

	//+kubebuilder:default="opster-cluster"
	ClusterName string `json:"clusterName,omitempty"`
	HttpPort    int32  `json:"httpPort,omitempty"`
	/////////+kubebuilder:validation:Enum=Opensearch,Elasticsearch,Op,Es,OP,ES
	Vendor         string `json:"vendor,omitempty"`
	Version        string `json:"version,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
	ServiceName    string `json:"serviceName,omitempty"`
}

type NodePool struct {
	Component    string   `json:"component,omitempty"`
	Replicas     int32    `json:"replicas,omitempty"`
	DiskSize     int32    `json:"diskSize,omitempty"`
	NodeSelector string   `json:"nodeSelector,omitempty"`
	Cpu          int32    `json:"cpu,omitempty"`
	Memory       int32    `json:"memory,omitempty"`
	Jvm          string   `json:"jvm,omitempty"`
	Roles        []string `json:"roles"`
}

// ConfMgmt defines which additional services will be deployed
type ConfMgmt struct {
	AutoScaler bool `json:"autoScaler,omitempty"`
	Monitoring bool `json:"monitoring,omitempty"`
	VerUpdate  bool `json:"VerUpdate,omitempty"`
}

type DashboardsConfig struct {
	Enable bool `json:"enable,omitempty"`
}

// ClusterSpec defines the desired state of OpenSearchCluster
type ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	General    GeneralConfig    `json:"general,omitempty"`
	ConfMgmt   ConfMgmt         `json:"confMgmt,omitempty"`
	Dashboards DashboardsConfig `json:"dashboards,omitempty"`
	NodePools  []NodePool       `json:"nodePools"`
}

// ClusterStatus defines the observed state of Es
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase            string             `json:"phase,omitempty"`
	ComponentsStatus []ComponentsStatus `json:"componentsStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=os;opensearch
// Es is the Schema for the es API
type OpenSearchCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

type ComponentsStatus struct {
	Component   string `json:"component,omitempty"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
}

//+kubebuilder:object:root=true
// EsList contains a list of Es
type OpenSearchClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenSearchCluster{}, &OpenSearchClusterList{})
}
