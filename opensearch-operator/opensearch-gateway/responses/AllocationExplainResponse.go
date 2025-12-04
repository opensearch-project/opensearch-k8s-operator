package responses

// AllocationExplainResponse represents the response from _cluster/allocation/explain API
type AllocationExplainResponse struct {
	Index            string                 `json:"index"`
	Shard            int                    `json:"shard"`
	Primary          bool                   `json:"primary"`
	CurrentState     string                 `json:"current_state"`
	CurrentNode      *AllocationExplainNode `json:"current_node,omitempty"`
	UnassignedInfo   *UnassignedInfo        `json:"unassigned_info,omitempty"`
	CanAllocate      string                 `json:"can_allocate"`
	AllocateDecision *AllocateDecision      `json:"allocate_decision,omitempty"`
}

// AllocationExplainNode represents a node in the allocation explain response
type AllocationExplainNode struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	TransportAddress string            `json:"transport_address"`
	Attributes       map[string]string `json:"attributes,omitempty"`
}

// UnassignedInfo contains information about why a shard is unassigned
type UnassignedInfo struct {
	Reason               string `json:"reason"`
	At                   string `json:"at"`
	Details              string `json:"details,omitempty"`
	LastAllocationStatus string `json:"last_allocation_status,omitempty"`
}

// AllocateDecision contains the decision and explanation for allocation
type AllocateDecision struct {
	Decider     string `json:"decider"`
	Decision    string `json:"decision"`
	Explanation string `json:"explanation"`
}
