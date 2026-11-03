package responses

// CatClusterManagerResponse holds one row identifying the elected cluster
// manager. opensearch-go v1 has no CatClusterManagerRequest, so it is fetched
// via /_cat/master — an alias returning identical data on OpenSearch 2.x.
type CatClusterManagerResponse struct {
	Id   string `json:"id"`
	Host string `json:"host"`
	Ip   string `json:"ip"`
	Node string `json:"node"`
}
