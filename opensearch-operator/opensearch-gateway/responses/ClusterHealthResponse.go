package responses

type ClusterHealthResponse struct {
	Status             string                 `json:"status,omitempty"`
	ActiveShards       int                    `json:"active_shards,omitempty"`
	RelocatingShards   int                    `json:"relocating_shards,omitempty"`
	InitializingShards int                    `json:"initializing_shards,omitempty"`
	UnassignedShards   int                    `json:"unassigned_shards,omitempty"`
	PercentActive      float32                `json:"active_shards_percent_as_number,omitempty"`
	Indices            map[string]IndexHealth `json:"indices,omitempty"`
}

type IndexHealth struct {
	Status              string `json:"status"`
	NumberOfShards      int    `json:"number_of_shards"`
	NumberOfReplicas    int    `json:"number_of_replicas"`
	ActivePrimaryShards int    `json:"active_primary_shards"`
	ActiveShards        int    `json:"active_shards"`
	RelocatingShards    int    `json:"relocating_shards"`
	InitializingShards  int    `json:"initializing_shards"`
	UnassignedShards    int    `json:"unassigned_shards"`
}
