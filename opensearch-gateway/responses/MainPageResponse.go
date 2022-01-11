package responses

type MainResponse struct {
	Name        string              `json:"name"`
	ClusterName string              `json:"cluster_name"`
	ClusterUuid string              `json:"cluster_uuid"`
	Version     MainResponseVersion `json:"version"`
	Tagline     string              `json:"tagline"`
}

type MainResponseVersion struct {
	Distribution                     string `json:"distribution"`
	Number                           string `json:"number"`
	BuildType                        string `json:"build_type"`
	BuildHash                        string `json:"build_hash"`
	BuildDate                        string `json:"build_date"`
	BuildSnapshot                    bool   `json:"build_snapshot"`
	LuceneVersion                    string `json:"lucene_version"`
	MinimumWireCompatibilityVersion  string `json:"minimum_wire_compatibility_version"`
	MinimumIndexCompatibilityVersion string `json:"minimum_index_compatibility_version"`
}
