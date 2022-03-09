package responses

type ClusterHealthResponse struct {
	Status             string  `json:"status,omitempty"`
	ActiveShards       int     `json:"active_shards,omitempty"`
	RelocatingShards   int     `json:"relocating_shards,omitempty"`
	InitializingShards int     `json:"initializing_shards,omitempty"`
	UnassignedShards   int     `json:"unassigned_shards,omitempty"`
	PercentActive      float32 `json:"active_shards_percent_as_number,omitempty"`
}
