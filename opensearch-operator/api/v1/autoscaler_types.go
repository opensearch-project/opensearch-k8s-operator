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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AutoscalerSpec defines the desired state of Autoscaler
type AutoscalerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Rules []Rule `json:"rules,omitempty"`
}

// Rule defines the contents of an autoscaler rule
type Rule struct {
	NodeRole string `json:"nodeRole"`
	Behavior Scale  `json:"behavior"`
	Items    []Item `json:"items"`
}

type Scale struct {
	ScaleUp   ScaleConf `json:"scaleUp,omitempty"`
	ScaleDown ScaleConf `json:"scaleDown,omitempty"`
}

type ScaleConf struct {
	Enable      bool  `json:"enable"`
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
}

// Item defines the contents of an autoscaler rule item
type Item struct {
	Metric       string       `json:"metric"`
	Operator     string       `json:"operator"`
	Threshold    string       `json:"threshold"`
	QueryOptions QueryOptions `json:"queryOptions,omitempty"`
}

type QueryOptions struct {
	LabelMatchers       []string `json:"labelMatchers,omitempty"`
	Function            string   `json:"function,omitempty"`
	Interval            string   `json:"interval,omitempty"`
	AggregateEvaluation bool     `json:"aggregateEvaluation,omitempty"`
}

// AutoscalerStatus defines the observed state of Autoscaler
type AutoscalerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Component   string `json:"component,omitempty"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// Autoscaler is the Schema for the autoscalers API
type Autoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoscalerSpec   `json:"spec,omitempty"`
	Status AutoscalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AutoscalerList contains a list of Autoscaler
type AutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Autoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Autoscaler{}, &AutoscalerList{})
}
