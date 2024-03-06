package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type OpensearchISMPolicyState string

const (
	OpensearchISMPolicyPending OpensearchISMPolicyState = "PENDING"
	OpensearchISMPolicyCreated OpensearchISMPolicyState = "CREATED"
	OpensearchISMPolicyError   OpensearchISMPolicyState = "ERROR"
	OpensearchISMPolicyIgnored OpensearchISMPolicyState = "IGNORED"
)

// OpensearchISMPolicyStatus defines the observed state of OpensearchISMPolicy
type OpensearchISMPolicyStatus struct {
	State             OpensearchISMPolicyState `json:"state,omitempty"`
	Reason            string                   `json:"reason,omitempty"`
	ExistingISMPolicy *bool                    `json:"existingISMPolicy,omitempty"`
	ManagedCluster    *types.UID               `json:"managedCluster,omitempty"`
	PolicyId          string                   `json:"policyId,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=ismp;ismpolicy
// +kubebuilder:subresource:status
type OpenSearchISMPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OpenSearchISMPolicySpec   `json:"spec,omitempty"`
	Status            OpensearchISMPolicyStatus `json:"status,omitempty"`
}

// ISMPolicySpec is the specification for the ISM policy for OS.
type OpenSearchISMPolicySpec struct {
	OpensearchRef corev1.LocalObjectReference `json:"opensearchCluster,omitempty"`
	// The default starting state for each index that uses this policy.
	DefaultState string `json:"defaultState"`
	// A human-readable description of the policy.
	Description       string             `json:"description"`
	ErrorNotification *ErrorNotification `json:"errorNotification,omitempty"`
	// Specify an ISM template pattern that matches the index to apply the policy.
	ISMTemplate *ISMTemplate `json:"ismTemplate,omitempty"`
	PolicyID    string       `json:"policyId,omitempty"`
	// The states that you define in the policy.
	States []State `json:"states"`
}

type ErrorNotification struct {
	// The destination URL.
	Destination *Destination `json:"destination,omitempty"`
	Channel     string       `json:"channel,omitempty"`
	// The text of the message
	MessageTemplate *MessageTemplate `json:"messageTemplate,omitempty"`
}

type Destination struct {
	Slack         *DestinationURL `json:"slack,omitempty"`
	Amazon        *DestinationURL `json:"amazon,omitempty"`
	Chime         *DestinationURL `json:"chime,omitempty"`
	CustomWebhook *DestinationURL `json:"customWebhook,omitempty"`
}

type DestinationURL struct {
	URL string `json:"url,omitempty"`
}

type MessageTemplate struct {
	Source string `json:"source,omitempty"`
}

type ISMTemplate struct {
	// Index patterns on which this policy has to be applied
	IndexPatterns []string `json:"indexPatterns"`
	// Priority of the template, defaults to 0
	Priority int `json:"priority,omitempty"`
}

type State struct {
	// The actions to execute after entering a state.
	Actions []Action `json:"actions"`
	// The name of the state.
	Name string `json:"name"`
	// The next states and the conditions required to transition to those states. If no transitions exist, the policy assumes that it’s complete and can now stop managing the index
	Transitions []Transition `json:"transitions,omitempty"`
}

// Actions are the steps that the policy sequentially executes on entering a specific state.
type Action struct {
	Alias *Alias `json:"alias,omitempty"`
	// Allocate the index to a node with a specific attribute set
	Allocation *Allocation `json:"allocation,omitempty"`
	// Closes the managed index.
	Close *Close `json:"close,omitempty"`
	// Deletes a managed index.
	Delete *Delete `json:"delete,omitempty"`
	// Reduces the number of Lucene segments by merging the segments of individual shards.
	ForceMerge *ForceMerge `json:"forceMerge,omitempty"`
	// Set the priority for the index in a specific state.
	IndexPriority *IndexPriority `json:"indexPriority,omitempty"`
	//Name          string        `json:"name,omitempty"`
	Notification *Notification `json:"notification,omitempty"`
	// Opens a managed index.
	Open *Open `json:"open,omitempty"`
	// Sets a managed index to be read only.
	ReadOnly *string `json:"readOnly,omitempty"`
	// Sets a managed index to be writeable.
	ReadWrite *string `json:"readWrite,omitempty"`
	// Sets the number of replicas to assign to an index.
	ReplicaCount *ReplicaCount `json:"replicaCount,omitempty"`
	// The retry configuration for the action.
	Retry *Retry `json:"retry,omitempty"`
	// Rolls an alias over to a new index when the managed index meets one of the rollover conditions.
	Rollover *Rollover `json:"rollover,omitempty"`
	// Periodically reduce data granularity by rolling up old data into summarized indexes.
	Rollup *Rollup `json:"rollup,omitempty"`
	// Allows you to reduce the number of primary shards in your indexes
	Shrink *Shrink `json:"shrink,omitempty"`
	// Back up your cluster’s indexes and state
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	// The timeout period for the action.
	Timeout *string `json:"timeout,omitempty"`
}

type Alias struct {
	// Allocate the index to a node with a specified attribute.
	Actions []AliasAction `json:"actions"`
}
type AliasAction struct {
	Add    *AliasDetails `json:"add,omitempty"`
	Remove *AliasDetails `json:"remove,omitempty"`
}

type AliasDetails struct {
	// The name of the index that the alias points to.
	Index *string `json:"index,omitempty"`
	// The name of the alias.
	Aliases []string `json:"aliases,omitempty"`
	// Limit search to an associated shard value
	Routing *string `json:"routing,omitempty"`
	// Specify the index that accepts any write operations to the alias.
	IsWriteIndex *bool `json:"isWriteIndex,omitempty"`
}

type Allocation struct {
	// Allocate the index to a node with a specified attribute.
	Exclude string `json:"exclude"`
	// Allocate the index to a node with any of the specified attributes.
	Include string `json:"include"`
	// Don’t allocate the index to a node with any of the specified attributes.
	Require string `json:"require"`
	// Wait for the policy to execute before allocating the index to a node with a specified attribute.
	WaitFor string `json:"waitFor"`
}

type Close struct{}

type Delete struct{}

type ForceMerge struct {
	// The number of segments to reduce the shard to.
	MaxNumSegments int64 `json:"maxNumSegments"`
}

type IndexPriority struct {
	// The priority for the index as soon as it enters a state.
	Priority int64 `json:"priority"`
}

type Notification struct {
	Destination     string          `json:"destination"`
	MessageTemplate MessageTemplate `json:"messageTemplate"`
}

type Open struct{}

type ReplicaCount struct {
	NumberOfReplicas int64 `json:"numberOfReplicas"`
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
	MinDocCount *int64 `json:"minDocCount,omitempty"`
	// The minimum age required to roll over the index.
	MinIndexAge *string `json:"minIndexAge,omitempty"`
	// The minimum storage size of a single primary shard required to roll over the index.
	MinPrimaryShardSize *string `json:"minPrimaryShardSize,omitempty"`
	// The minimum size of the total primary shard storage (not counting replicas) required to roll over the index.
	MinSize *string `json:"minSize,omitempty"`
}

type Rollup struct{}

type Shrink struct {
	// If true, executes the shrink action even if there are no replicas.
	ForceUnsafe *bool `json:"forceUnsafe,omitempty"`
	// The maximum size in bytes of a shard for the target index.
	MaxShardSize *string `json:"maxShardSize,omitempty"`
	// The maximum number of primary shards in the shrunken index.
	NumNewShards *int `json:"numNewShards,omitempty"`
	// Percentage of the number of original primary shards to shrink.
	PercentageOfSourceShards *int64 `json:"percentageOfSourceShards,omitempty"`
	// The name of the shrunken index.
	TargetIndexNameTemplate *string `json:"targetIndexNameTemplate,omitempty"`
}

type Snapshot struct {
	// The repository name that you register through the native snapshot API operations.
	Repository string `json:"repository"`
	// The name of the snapshot.
	Snapshot string `json:"snapshot"`
}

type Transition struct {
	// conditions for the transition.
	Conditions Condition `json:"conditions"`
	// The name of the state to transition to if the conditions are met.
	StateName string `json:"stateName"`
}

type Condition struct {
	// The cron job that triggers the transition if no other transition happens first.
	Cron *Cron `json:"cron,omitempty"`
	// The minimum document count of the index required to transition.
	MinDocCount *int64 `json:"minDocCount,omitempty"`
	// The minimum age of the index required to transition.
	MinIndexAge *string `json:"minIndexAge,omitempty"`
	// The minimum age required after a rollover has occurred to transition to the next state.
	MinRolloverAge *string `json:"minRolloverAge,omitempty"`
	// The minimum size of the total primary shard storage (not counting replicas) required to transition.
	MinSize *string `json:"minSize,omitempty"`
}

type Cron struct {
	// The cron expression that triggers the transition.
	Expression string `json:"expression"`
	// The timezone that triggers the transition.
	Timezone string `json:"timezone"`
}

// +kubebuilder:object:root=true
// ISMPolicyList contains a list of ISMPolicy
type OpenSearchISMPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchISMPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenSearchISMPolicy{}, &OpenSearchISMPolicyList{})
}
