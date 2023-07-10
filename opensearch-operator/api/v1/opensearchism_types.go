package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=ismp;ismpolicy
type ISMPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ISMPolicySpec `json:"spec,omitempty"`
}

// ISMPolicySpec is the specification for the ISM policy for OS.
type ISMPolicySpec struct {
	// The default starting state for each index that uses this policy.
	DefaultState string `json:"default_state"`
	// A human-readable description of the policy.
	Description       string            `json:"description"`
	ErrorNotification ErrorNotification `json:"error_notification,omitempty"`
	// Specify an ISM template pattern that matches the index to apply the policy.
	ISMTemplate ISMTemplate `json:"ism_template,omitempty"`
	PolicyID    string      `json:"policy_id,omitempty"`
	// The time the policy was last updated.
	LastUpdatedTime string `json:"last_updated_time,omitempty"`
	// The states that you define in the policy.
	States []State `json:"states"`
}

type ErrorNotification struct {
	// The destination URL.
	Destination string `json:"destination,omitempty"`
	Channel     string `json:"channel,omitempty"`
	// The text of the message
	MessageTemplate MessageTemplate `json:"message_template,omitempty"`
}

type MessageTemplate struct {
	Source string `json:"source,omitempty"`
}

type ISMTemplate struct {
	// Index patterns on which this policy has to be applied
	IndexPatterns []string `json:"index_patterns"`
	// Priority of the template, defaults to 0
	Priority int `json:"priority,omitempty"`
}

type State struct {
	// The actions to execute after entering a state.
	Actions []Action `json:"actions"`
	// The name of the state.
	Name string `json:"name"`
	// The next states and the conditions required to transition to those states. If no transitions exist, the policy assumes that it’s complete and can now stop managing the index
	Transitions []Transition `json:"transitions"`
}

// Actions are the steps that the policy sequentially executes on entering a specific state.
type Action struct {
	// Allocate the index to a node with a specific attribute set
	Allocation Allocation `json:"allocation,omitempty"`
	// Closes the managed index.
	Close *Close `json:"close,omitempty"`
	// Deletes a managed index.
	Delete *Delete `json:"delete,omitempty"`
	// Reduces the number of Lucene segments by merging the segments of individual shards.
	ForceMerge ForceMerge `json:"force_merge,omitempty"`
	// Set the priority for the index in a specific state.
	IndexPriority IndexPriority `json:"index_priority,omitempty"`
	Name          string        `json:"name,omitempty"`
	Notification  Notification  `json:"notification,omitempty"`
	// Opens a managed index.
	Open *Open `json:"open,omitempty"`
	// Sets a managed index to be read only.
	ReadOnly string `json:"read_only,omitempty"`
	// Sets a managed index to be writeable.
	ReadWrite string `json:"read_write,omitempty"`
	// Sets the number of replicas to assign to an index.
	ReplicaCount ReplicaCount `json:"replica_count,omitempty"`
	// The retry configuration for the action.
	Retry Retry `json:"retry,omitempty"`
	// Rolls an alias over to a new index when the managed index meets one of the rollover conditions.
	Rollover Rollover `json:"rollover,omitempty"`
	// Periodically reduce data granularity by rolling up old data into summarized indexes.
	Rollup *Rollup `json:"rollup,omitempty"`
	// Allows you to reduce the number of primary shards in your indexes
	Shrink Shrink `json:"shrink,omitempty"`
	// Back up your cluster’s indexes and state
	Snapshot Snapshot `json:"snapshot,omitempty"`
	// The timeout period for the action.
	Timeout string `json:"timeout,omitempty"`
}

type Allocation struct {
	// Allocate the index to a node with a specified attribute.
	Exclude string `json:"exclude"`
	// Allocate the index to a node with any of the specified attributes.
	Include string `json:"include"`
	// Don’t allocate the index to a node with any of the specified attributes.
	Require string `json:"require"`
	// Wait for the policy to execute before allocating the index to a node with a specified attribute.
	WaitFor string `json:"wait_for"`
}

type Close struct{}

type Delete struct{}

type ForceMerge struct {
	// The number of segments to reduce the shard to.
	MaxNumSegments int64 `json:"max_num_segments"`
}

type IndexPriority struct {
	// The priority for the index as soon as it enters a state.
	Priority int64 `json:"priority"`
}

type Notification struct {
	Destination     string          `json:"destination"`
	MessageTemplate MessageTemplate `json:"message_template"`
}

type Open struct{}

type ReplicaCount struct {
	NumberOfReplicas int64 `json:"number_of_replicas"`
}

type Retry struct {
	// The backoff policy type to use when retrying.
	Backoff string `json:"backoff,omitempty"`
	// The number of retry counts.
	Count int64 `json:"count"`
	// The time to wait between retries.
	Delay string `json:"delay,omitempty"`
}

type Rollover struct {
	// The minimum number of documents required to roll over the index.
	MinDocCount int64 `json:"min_doc_count,omitempty"`
	// The minimum age required to roll over the index.
	MinIndexAge string `json:"min_index_age,omitempty"`
	// The minimum storage size of a single primary shard required to roll over the index.
	MinPrimaryShardSize string `json:"min_primary_shard_size,omitempty"`
	// The minimum size of the total primary shard storage (not counting replicas) required to roll over the index.
	MinSize string `json:"min_size,omitempty"`
}

type Rollup struct{}

type Shrink struct {
	// If true, executes the shrink action even if there are no replicas.
	ForceUnsafe bool `json:"force_unsafe,omitempty"`
	// The maximum size in bytes of a shard for the target index.
	MaxShardSize string `json:"max_shard_size"`
	// The maximum number of primary shards in the shrunken index.
	NumNewShards int `json:"num_new_shards,omitempty"`
	// Percentage of the number of original primary shards to shrink.
	PercentageOfSourceShards int64 `json:"percentage_of_source_shards,omitempty"`
	// The name of the shrunken index.
	TargetIndexNameTemplate string `json:"target_index_name_template,omitempty"`
}

type Snapshot struct {
	// The repository name that you register through the native snapshot API operations.
	Repository string `json:"repository"`
	// The name of the snapshot.
	Snapshot string `json:"snapshot"`
}

type Transition struct {
	// conditions for the transition.
	Conditions []Condition `json:"conditions"`
	// The name of the state to transition to if the conditions are met.
	StateName string `json:"state_name"`
}

type Condition struct {
	// The cron job that triggers the transition if no other transition happens first.
	Cron Cron `json:"cron,omitempty"`
	// The minimum document count of the index required to transition.
	MinDocCount int64 `json:"min_doc_count,omitempty"`
	// The minimum age of the index required to transition.
	MinIndexAge string `json:"min_index_age,omitempty"`
	// The minimum age required after a rollover has occurred to transition to the next state.
	MinRolloverAge string `json:"min_rollover_age,omitempty"`
	// The minimum size of the total primary shard storage (not counting replicas) required to transition.
	MinSize string `json:"min_size,omitempty"`
}

type Cron struct {
	// The cron expression that triggers the transition.
	Expression string `json:"expression"`
	// The timezone that triggers the transition.
	Timezone string `json:"timezone"`
}

// +kubebuilder:object:root=true
// ISMPolicyList contains a list of ISMPolicy
type ISMPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ISMPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ISMPolicy{}, &ISMPolicyList{})
}
