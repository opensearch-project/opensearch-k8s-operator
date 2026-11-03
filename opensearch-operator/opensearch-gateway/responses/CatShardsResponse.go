package responses

type CatShardsResponse struct {
	Index             string `json:"index"`
	Shard             string `json:"shard"`
	PrimaryOrReplica  string `json:"prirep"`
	State             string `json:"state"`
	Docs              string `json:"docs"`
	Store             string `json:"store"`
	Ip                string `json:"ip"`
	NodeName          string `json:"node"`
	NodeId            string `json:"id"`
	UnassignedAt      string `json:"unassigned.at"`
	UnassignedDetails string `json:"unassigned.details"`
	UnassignedFor     string `json:"unassigned.for"`
	UnassignedReason  string `json:"unassigned.reason"`
	CompletionSize    string `json:"completion.size"`
}
