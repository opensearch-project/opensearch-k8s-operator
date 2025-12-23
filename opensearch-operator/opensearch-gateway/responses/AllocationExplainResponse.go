package responses

// AllocationExplainResponse represents the response from _cluster/allocation/explain API
type AllocationExplainResponse struct {
	Index                   string                          `json:"index"`
	Shard                   int                             `json:"shard"`
	Primary                 bool                            `json:"primary"`
	CurrentState            string                          `json:"current_state"`
	CurrentNode             *AllocationExplainNode          `json:"current_node,omitempty"`               // only if assigned
	CanRemainOnCurrentNode  string                          `json:"can_remain_on_current_node,omitempty"` // only if assigned
	CanMoveToOtherNode      string                          `json:"can_move_to_other_node,omitempty"`     // only if assigned
	UnassignedInfo          *UnassignedInfo                 `json:"unassigned_info,omitempty"`            // only if unassigned
	CanAllocate             string                          `json:"can_allocate,omitempty"`               // only if unassigned
	NodeAllocationDecisions []AllocationExplainNodeDecision `json:"node_allocation_decisions"`
}

// AllocationExplainNode represents the current node in the allocation explain response
type AllocationExplainNode struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	TransportAddress string            `json:"transport_address"`
	Attributes       map[string]string `json:"attributes,omitempty"`
}

// AllocationExplainNodeDecision represents a node decision in the allocation explain response
type AllocationExplainNodeDecision struct {
	ID               string               `json:"node_id"`
	Name             string               `json:"node_name"`
	TransportAddress string               `json:"transport_address"`
	Attributes       map[string]string    `json:"attributes,omitempty"`
	Decision         string               `json:"node_decision"`
	Deciders         []AllocationDecision `json:"deciders,omitempty"`
}

// UnassignedInfo contains information about why a shard is unassigned
type UnassignedInfo struct {
	Reason               string `json:"reason"`
	At                   string `json:"at"`
	Details              string `json:"details,omitempty"`
	LastAllocationStatus string `json:"last_allocation_status,omitempty"`
}

// AllocationDecision contains the decision and explanation for allocation
type AllocationDecision struct {
	Decider     string `json:"decider"`
	Decision    string `json:"decision"`
	Explanation string `json:"explanation"`
}
