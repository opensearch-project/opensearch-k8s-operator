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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending = "PENDING"
	PhaseRunning = "RUNNING"
	PhaseDone    = "DONE"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type EsGeneral struct {

	//+kubebuilder:default="Opster_cluster"
	ClusterName string `json:"clusterName,omitempty"`
	EsPort      int32  `json:"esPort,omitempty"`
	/////////+kubebuilder:validation:Enum=Opensearch,Elasticsearch,Op,Es,OP,ES
	Vendor         string `json:"vendor,omitempty"`
	Version        string `json:"version,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
	ServiceName    string `json:"serviceName,omitempty"`
}

type EsNodes struct {
	Replicas     int32  `json:"replicas,omitempty"`
	DiskSize     int32  `json:"diskSize,omitempty"`
	NodeSelector string `json:"nodeSelector,omitempty"`
	Cpu          int32  `json:"cpu,omitempty"`
	Memory       int32  `json:"memory,omitempty"`
	Ingest       string `json:"ingest,omitempty"`
	Jvm          string `json:"jvm,omitempty"`
}

type EsMasters struct {
	/////////+kubebuilder:validation:Enum=3,5
	Replicas     int32  `json:"replicas,omitempty"`
	DiskSize     int32  `json:"diskSize,omitempty"`
	NodeSelector string `json:"nodeSelector,omitempty"`
	Cpu          int32  `json:"cpu,omitempty"`
	Memory       int32  `json:"memory,omitempty"`
	Jvm          string `json:"jvm,omitempty"`
}

type EsConf struct {
}

// EsSpec defines the desired state of Es
type EsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	//
	//// Foo is an example field of Es. Edit es_types.go to remove/update

	General EsGeneral `json:"general,omitempty"`
	Masters EsMasters `json:"masters,omitempty"`
	Nodes   EsNodes   `json:"nodes,omitempty"`
}

// EsStatus defines the observed state of Es
type EsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Phase string `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Es is the Schema for the es API
type Es struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EsSpec   `json:"spec,omitempty"`
	Status EsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EsList contains a list of Es
type EsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Es `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Es{}, &EsList{})
}
