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

// NOTE: Add or update CRD fields below to introduce new features or modify functionality.
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type OpensearchSnapshotPolicyState string

const (
	OpensearchSnapshotPolicyPending OpensearchSnapshotPolicyState = "PENDING"
	OpensearchSnapshotPolicyCreated OpensearchSnapshotPolicyState = "CREATED"
	OpensearchSnapshotPolicyError   OpensearchSnapshotPolicyState = "ERROR"
	OpensearchSnapshotPolicyIgnored OpensearchSnapshotPolicyState = "IGNORED"
)

type OpensearchSnapshotPolicySpec struct {
	OpensearchRef  corev1.LocalObjectReference `json:"opensearchCluster"`
	PolicyName     string                      `json:"policyName"`
	Description    *string                     `json:"description,omitempty"`
	Enabled        *bool                       `json:"enabled,omitempty"`
	SnapshotConfig SnapshotConfig              `json:"snapshotConfig"`
	Creation       SnapshotCreation            `json:"creation"`
	Deletion       *SnapshotDeletion           `json:"deletion,omitempty"`
	Notification   *SnapshotNotification       `json:"notification,omitempty"`
}

type SnapshotConfig struct {
	DateFormat         string            `json:"dateFormat,omitempty"`
	DateFormatTimezone string            `json:"dateFormatTimezone,omitempty"`
	Indices            string            `json:"indices,omitempty"`
	Repository         string            `json:"repository"`
	IgnoreUnavailable  bool              `json:"ignoreUnavailable,omitempty"`
	IncludeGlobalState bool              `json:"includeGlobalState,omitempty"`
	Partial            bool              `json:"partial,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type SnapshotCreation struct {
	Schedule  CronSchedule `json:"schedule"`
	TimeLimit *string      `json:"timeLimit,omitempty"`
}

type CronSchedule struct {
	Cron CronExpression `json:"cron"`
}

type CronExpression struct {
	Expression string `json:"expression"`
	Timezone   string `json:"timezone"`
}

type SnapshotDeletion struct {
	Schedule        *CronSchedule            `json:"schedule,omitempty"`
	TimeLimit       *string                  `json:"timeLimit,omitempty"`
	DeleteCondition *SnapshotDeleteCondition `json:"deleteCondition,omitempty"`
}

type SnapshotDeleteCondition struct {
	MaxCount *int    `json:"maxCount,omitempty"`
	MaxAge   *string `json:"maxAge,omitempty"`
	MinCount *int    `json:"minCount,omitempty"`
}

type SnapshotNotification struct {
	Channel    NotificationChannel     `json:"channel"`
	Conditions *NotificationConditions `json:"conditions,omitempty"`
}

type NotificationChannel struct {
	ID string `json:"id"`
}

type NotificationConditions struct {
	Creation *bool `json:"creation,omitempty"`
	Deletion *bool `json:"deletion,omitempty"`
	Failure  *bool `json:"failure,omitempty"`
}

// OpensearchSnapshotPolicyStatus defines the observed state of OpensearchSnapshotPolicy
type OpensearchSnapshotPolicyStatus struct {
	State                  OpensearchSnapshotPolicyState `json:"state,omitempty"`
	Reason                 string                        `json:"reason,omitempty"`
	SnapshotPolicyName     string                        `json:"snapshotPolicyName,omitempty"`
	ManagedCluster         *types.UID                    `json:"managedCluster,omitempty"`
	ExistingSnapshotPolicy *bool                         `json:"existingSnapshotPolicy,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OpensearchSnapshotPolicy is the Schema for the opensearchsnapshotpolicies API
// +kubebuilder:printcolumn:name="existingpolicy",type="boolean",JSONPath=".status.existingSnapshotPolicy",description="Existing policy state"
// +kubebuilder:printcolumn:name="policyName",type="string",JSONPath=".status.snapshotPolicyName",description="Snapshot policy name"
// +kubebuilder:printcolumn:name="state",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type OpensearchSnapshotPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchSnapshotPolicySpec   `json:"spec,omitempty"`
	Status OpensearchSnapshotPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpensearchSnapshotPolicyList contains a list of OpensearchSnapshotPolicy
type OpensearchSnapshotPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchSnapshotPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchSnapshotPolicy{}, &OpensearchSnapshotPolicyList{})
}
