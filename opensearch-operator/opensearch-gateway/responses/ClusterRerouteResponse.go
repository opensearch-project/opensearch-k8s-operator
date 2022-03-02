package responses

type ClusterRerouteResponse struct {
	Acknowledged bool                        `json:"acknowledged"`
	State        ClusterRerouteStateResponse `json:"state"`
}

type ClusterRerouteStateResponse struct {
	ClusterUuid  string                                               `json:"cluster_uuid"`
	Version      int32                                                `json:"version"`
	StateUuid    string                                               `json:"state_uuid"`
	MasterNode   string                                               `json:"master_node"`
	Blocks       map[string]string                                    `json:"blocks"`
	Nodes        map[string]ClusterRerouteStateNodeResponse           `json:"nodes"`
	RoutingTable map[string]ClusterRerouteStateRoutingTableResponse   `json:"routing_table"`
	RoutingNodes map[string][]ClusterRerouteStateRoutingNodesResponse `json:"routing_nodes"`
}

type ClusterRerouteStateNodeResponse struct {
	Name             string            `json:"name"`
	EphemeralId      int32             `json:"ephemeral_id"`
	StateUuid        string            `json:"state_uuid"`
	TransportAddress string            `json:"transport_address"`
	Attributes       map[string]string `json:"attributes"`
}

type ClusterRerouteStateRoutingTableResponse struct {
	Indices map[string]ClusterRerouteStateRoutingTableIndicesShardsResponse `json:"indices"`
}
type ClusterRerouteStateRoutingNodesResponse struct {
	Unassigned []ClusterRerouteStateRoutingNodesUnAssignedResponse                         `json:"unassigned"`
	Nodes      map[string][]ClusterRerouteStateRoutingTableIndicesShardsAllocationResponse `json:"nodes"`
}

type ClusterRerouteStateRoutingTableIndicesShardsResponse struct {
	Shards map[string][]ClusterRerouteStateRoutingTableIndicesShardsAllocationResponse `json:"shards"`
}

type ClusterRerouteStateRoutingTableIndicesShardsAllocationResponse struct {
	Index            string            `json:"index"`
	Shard            int32             `json:"shard"`
	PrimaryOrReplica bool              `json:"primary"`
	State            string            `json:"state"`
	Node             string            `json:"node"`
	RelocatingNode   string            `json:"relocating_node"`
	AllocationId     map[string]string `json:"allocation_id"`
}

type ClusterRerouteStateRoutingNodesUnAssignedResponse struct {
	Index            string            `json:"index"`
	Shard            int32             `json:"shard"`
	PrimaryOrReplica bool              `json:"primary"`
	State            string            `json:"state"`
	Node             string            `json:"node"`
	RelocatingNode   string            `json:"relocating_node"`
	RecoverySource   map[string]string `json:"recovery_source"`
}

type ClusterRerouteStateRoutingNodesUnAssignedInfoResponse struct {
	Reason           string   `json:"reason"`
	At               string   `json:"at"`
	FailedAttempts   int32    `json:"failed_attempts"`
	FailedNodes      []string `json:"failed_nodes"`
	Delayed          bool     `json:"delayed"`
	Details          string   `json:"details"`
	AllocationStatus string   `json:"allocation_status"`
}
