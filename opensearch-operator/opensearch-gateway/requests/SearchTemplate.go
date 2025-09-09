package requests

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	OpensearchSearchTemplateLang = "mustache"
)

// OpensearchSearchTemplateSpec defines the desired state of OpensearchSearchTemplate
type SearchTemplateSpec struct {
	ScriptId string               `json:"id"`
	Script   SearchTemplateScript `json:"script"`
	Params   apiextv1.JSON        `json:"params"`
}

type SearchTemplateScript struct {
	// Lang is not set explicitly for Search Template because it must always be "mustache"
	AllowNoIndices             *bool         `json:"allow_no_indices,omitempty"`              // Ignore wildcards that don't match any indexes. Default: true
	AllowPartialSearchResults  *bool         `json:"allow_partial_search_results,omitempty"`  // Return partial results on errors or timeouts. Default: true
	Analyzer                   *string       `json:"analyzer,omitempty"`                      // Analyzer to use in the query string
	AnalyzeWildcard            *bool         `json:"analyze_wildcard,omitempty"`              // Include wildcard/prefix queries in analysis. Default: false
	BatchedReduceSize          *int          `json:"batched_reduce_size,omitempty"`           // Number of shard results to reduce on a node. Default: 512
	CancelAfterTimeInterval    *string       `json:"cancel_after_time_interval,omitempty"`    // Time after which search request is canceled. Default: -1
	CCSMinimizeRoundtrips      *bool         `json:"ccs_minimize_roundtrips,omitempty"`       // Minimize roundtrips to remote clusters. Default: true
	DefaultOperator            *string       `json:"default_operator,omitempty"`              // Default string query operator: AND or OR. Default: OR
	DF                         *string       `json:"df,omitempty"`                            // Default field when prefix is not provided
	DocvalueFields             []string      `json:"docvalue_fields,omitempty"`               // Fields to return using docvalue forms
	ExpandWildcards            *string       `json:"expand_wildcards,omitempty"`              // Types of indexes wildcard expressions can match. Default: open
	Explain                    *bool         `json:"explain,omitempty"`                       // Return details about how score was computed. Default: false
	From                       *int          `json:"from,omitempty"`                          // Starting index to search from. Default: 0
	IgnoreThrottled            *bool         `json:"ignore_throttled,omitempty"`              // Ignore frozen indexes. Default: true
	IgnoreUnavailable          *bool         `json:"ignore_unavailable,omitempty"`            // Include missing/closed indexes and ignore unavailable shards. Default: false
	Lenient                    *bool         `json:"lenient,omitempty"`                       // Accept malformed queries. Default: false
	MaxConcurrentShardRequests *int          `json:"max_concurrent_shard_requests,omitempty"` // Max concurrent shard requests per node. Default: 5
	PhaseTook                  *bool         `json:"phase_took,omitempty"`                    // Return phase-level took time values. Default: false
	PreFilterShardSize         *int          `json:"pre_filter_shard_size,omitempty"`         // Prefilter threshold by shard count. Default: 128
	Preference                 *string       `json:"preference,omitempty"`                    // Specify shards/nodes to perform search on
	Q                          *string       `json:"q,omitempty"`                             // Lucene query string
	RequestCache               *bool         `json:"request_cache,omitempty"`                 // Use request cache based on index setting
	RestTotalHitsAsInt         *bool         `json:"rest_total_hits_as_int,omitempty"`        // Return total hits as int. Default: false
	Routing                    *string       `json:"routing,omitempty"`                       // Shard routing value
	Scroll                     *string       `json:"scroll,omitempty"`                        // Duration to keep search context open
	SearchType                 *string       `json:"search_type,omitempty"`                   // Type of search: query_then_fetch or dfs_query_then_fetch. Default: query_then_fetch
	SeqNoPrimaryTerm           *bool         `json:"seq_no_primary_term,omitempty"`           // Include seq no and primary term. Default: false
	Size                       *int          `json:"size,omitempty"`                          // Number of results to return
	Sort                       []string      `json:"sort,omitempty"`                          // Sort fields in the format <field>:<direction>
	Source                     *string       `json:"_source,omitempty"`                       // Include _source field in response
	SourceExcludes             []string      `json:"_source_excludes,omitempty"`              // Source fields to exclude
	SourceIncludes             []string      `json:"_source_includes,omitempty"`              // Source fields to include
	Stats                      *string       `json:"stats,omitempty"`                         // Value to associate with request for logging
	StoredFields               []string      `json:"stored_fields,omitempty"`                 // Fields to retrieve from the index
	SuggestField               *string       `json:"suggest_field,omitempty"`                 // Field to use for suggestions
	SuggestMode                *string       `json:"suggest_mode,omitempty"`                  // Suggestion mode: always, popular, or missing
	SuggestSize                *int          `json:"suggest_size,omitempty"`                  // Number of suggestions to return
	SuggestText                *string       `json:"suggest_text,omitempty"`                  // Text to base suggestions on
	TerminateAfter             *int          `json:"terminate_after,omitempty"`               // Max matching docs before terminating search. Default: 0
	Timeout                    *string       `json:"timeout,omitempty"`                       // Time to wait for shard responses. Default: 1m
	TrackScores                *bool         `json:"track_scores,omitempty"`                  // Return document scores. Default: false
	TrackTotalHits             apiextv1.JSON `json:"track_total_hits,omitempty"`              // Track total hits: true, false, or integer threshold
	TypedKeys                  *bool         `json:"typed_keys,omitempty"`                    // Include types in aggregation/suggestion keys. Default: true
	Version                    *bool         `json:"version,omitempty"`                       // Include document version. Default: false
	IncludeNamedQueriesScore   *bool         `json:"include_named_queries_score,omitempty"`   // Return scores with named queries. Default: false
}
