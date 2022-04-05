package responses

type ClusterSettingsResponse struct {
	Persistent map[string]interface{} `json:"persistent,omitempty"`
	Transient  map[string]interface{} `json:"transient,omitempty"`
}

type FlatClusterSettingsResponse struct {
	Persistent Settings `json:"persistent,omitempty"`
	Transient  Settings `json:"transient,omitempty"`
}

type Settings struct {
	ClusterRoutingAllocationEnable  string `json:"cluster.routing.allocation.enable,omitempty"`
	ClusterRoutingAllocationExclude string `json:"cluster.routing.allocation.exclude._name,omitempty"`
}
