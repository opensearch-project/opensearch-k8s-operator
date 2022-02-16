package responses

type ClusterSettingsResponse struct {
	Persistent map[string]interface{} `json:"persistent,omitempty"`
	Transient  map[string]interface{} `json:"transient,omitempty"`
}
