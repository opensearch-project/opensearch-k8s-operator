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
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NOTE: Add or update CRD fields below to introduce new features or modify functionality.
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type OpensearchSearchTemplateState string

const (
	OpensearchSearchTemplatePending OpensearchSearchTemplateState = "PENDING"
	OpensearchSearchTemplateCreated OpensearchSearchTemplateState = "CREATED"
	OpensearchSearchTemplateError   OpensearchSearchTemplateState = "ERROR"
	OpensearchSearchTemplateIgnored OpensearchSearchTemplateState = "IGNORED"
)

// OpensearchSearchTemplateSpec defines the desired state of the search template.
type OpensearchSearchTemplateSpec struct {
	// Reference to the OpenSearch cluster to which this search template belongs.
	OpensearchRef corev1.LocalObjectReference `json:"opensearchCluster"`

	// ID under which the script/search template will be stored in OpenSearch.
	//+kubebuilder:validation:MinLength=1
	ScriptId string `json:"scriptId"`

	// Search template script definition and configuration.
	//+kubebuilder:validation:Required
	Script OpensearchSearchTemplateScript `json:"script"`

	// Parameters to be passed when executing the search template (dynamic variables).
	//+kubebuilder:validation:Required
	Params apiextv1.JSON `json:"params"`
}

// OpensearchSearchTemplateScript defines the body and query behavior of the search template.
type OpensearchSearchTemplateScript struct {
	// Whether to ignore if the index doesn't exist.
	AllowNoIndices *bool `json:"allowNoIndices,omitempty"`

	// Whether to allow partial search results if some shards fail.
	AllowPartialSearchResults *bool `json:"allowPartialSearchResults,omitempty"`

	// The analyzer to use for the query string.
	Analyzer *string `json:"analyzer,omitempty"`

	// Whether to analyze wildcard terms in the query string.
	AnalyzeWildcard *bool `json:"analyzeWildcard,omitempty"`

	// Maximum number of concurrent shard requests during reduce phase.
	BatchedReduceSize *int `json:"batchedReduceSize,omitempty"`

	// Cancel the search after a given time interval.
	CancelAfterTimeInterval *string `json:"cancelAfterTimeInterval,omitempty"`

	// Minimize cross-cluster search roundtrips.
	CCSMinimizeRoundtrips *bool `json:"ccsMinimizeRoundtrips,omitempty"`

	// Default boolean operator (AND/OR) for query strings.
	DefaultOperator *string `json:"defaultOperator,omitempty"`

	// Default field for query string search.
	DF *string `json:"df,omitempty"`

	// Fields to return in the `docvalue_fields` section.
	DocvalueFields []string `json:"docvalueFields,omitempty"`

	// Which wildcard expressions to expand (e.g., open, closed).
	ExpandWildcards *string `json:"expandWildcards,omitempty"`

	// Whether to include explanation of score in results.
	Explain *bool `json:"explain,omitempty"`

	// Starting offset for paginated search results.
	From *int `json:"from,omitempty"`

	// Whether to ignore throttled indices.
	IgnoreThrottled *bool `json:"ignoreThrottled,omitempty"`

	// Whether to ignore unavailable indices (closed or missing).
	IgnoreUnavailable *bool `json:"ignoreUnavailable,omitempty"`

	// Whether to allow queries with format errors.
	Lenient *bool `json:"lenient,omitempty"`

	// Max concurrent shard-level search requests.
	MaxConcurrentShardRequests *int `json:"maxConcurrentShardRequests,omitempty"`

	// Whether to collect per-phase timings.
	PhaseTook *bool `json:"phaseTook,omitempty"`

	// Pre-filter shards if number of hits is expected to be small.
	PreFilterShardSize *int `json:"preFilterShardSize,omitempty"`

	// Search preference (e.g., _primary, _replica, or custom string).
	Preference *string `json:"preference,omitempty"`

	// Query string.
	Q *string `json:"q,omitempty"`

	// Whether to use the request cache.
	RequestCache *bool `json:"requestCache,omitempty"`

	// Whether to return `total_hits` as integer.
	RestTotalHitsAsInt *bool `json:"restTotalHitsAsInt,omitempty"`

	// Custom routing value to control which shards to query.
	Routing *string `json:"routing,omitempty"`

	// Duration for scroll context to remain alive.
	Scroll *string `json:"scroll,omitempty"`

	// Type of search: query_then_fetch, dfs_query_then_fetch, etc.
	SearchType *string `json:"searchType,omitempty"`

	// Whether to include sequence number and primary term in results.
	SeqNoPrimaryTerm *bool `json:"seqNoPrimaryTerm,omitempty"`

	// Number of hits to return.
	Size *int `json:"size,omitempty"`

	// Fields to sort results on.
	Sort []string `json:"sort,omitempty"`

	// The query `_source` clause for including/excluding fields.
	Source *string `json:"source,omitempty"`

	// Fields to exclude from the source.
	SourceExcludes []string `json:"sourceExcludes,omitempty"`

	// Fields to include from the source.
	SourceIncludes []string `json:"sourceIncludes,omitempty"`

	// Stats groups to associate with this request.
	Stats *string `json:"stats,omitempty"`

	// List of stored fields to return (vs _source).
	StoredFields []string `json:"storedFields,omitempty"`

	// Field for suggestions (autocomplete).
	SuggestField *string `json:"suggestField,omitempty"`

	// Suggestion mode: missing, popular, always.
	SuggestMode *string `json:"suggestMode,omitempty"`

	// Number of suggestions to return.
	SuggestSize *int `json:"suggestSize,omitempty"`

	// Input text for suggestions.
	SuggestText *string `json:"suggestText,omitempty"`

	// Max number of documents to collect before terminating the query.
	TerminateAfter *int `json:"terminateAfter,omitempty"`

	// Timeout for the entire search request.
	Timeout *string `json:"timeout,omitempty"`

	// Whether to track scores even when not sorting by score.
	TrackScores *bool `json:"trackScores,omitempty"`

	// Track total number of hits (true/false or integer threshold).
	TrackTotalHits apiextv1.JSON `json:"trackTotalHits,omitempty"`

	// Whether response should include typed keys.
	TypedKeys *bool `json:"typedKeys,omitempty"`

	// Whether to include document version in hits.
	Version *bool `json:"version,omitempty"`

	// Include scores for named queries (not widely used).
	IncludeNamedQueriesScore *bool `json:"includeNamedQueriesScore,omitempty"`
}

// OpensearchSearchTemplateStatus defines the current state of the search template.
type OpensearchSearchTemplateStatus struct {
	// Current state (e.g., CREATED, PENDING, ERROR, IGNORED).
	State OpensearchSearchTemplateState `json:"state,omitempty"`

	// Reason for current state, if applicable.
	Reason string `json:"reason,omitempty"`

	// Name of the created search template in OpenSearch.
	SearchTemplateName string `json:"searchTemplateName,omitempty"`

	// UID of the managed OpenSearch cluster (used for multi-cluster context).
	ManagedCluster *types.UID `json:"managedCluster,omitempty"`

	// Whether the search template was already present in OpenSearch.
	ExistingSearchTemplate *bool `json:"existingSearchTemplate,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpensearchSearchTemplate is the Schema for the opensearchsearchtemplates API
type OpensearchSearchTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpensearchSearchTemplateSpec   `json:"spec,omitempty"`
	Status OpensearchSearchTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpensearchSearchTemplateList contains a list of OpensearchSearchTemplate
type OpensearchSearchTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpensearchSearchTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpensearchSearchTemplate{}, &OpensearchSearchTemplateList{})
}
