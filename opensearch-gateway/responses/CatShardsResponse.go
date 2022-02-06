package responses

type CatShardsResponse struct {
	Index             string `json:"index" json:"i" json:"idx"`
	Shard             string `json:"shard" json:"s" json:"sh"`
	PrimaryOrReplica  string `json:"prirep" json:"p" json:"pr" json:"primaryOrReplica"`
	State             string `json:"state" json:"st"`
	Docs              string `json:"docs" json:"d" json:"dc"`
	Store             string `json:"store" json:"sto"`
	Ip                string `json:"ip"`
	NodeName          string `json:"node" json:"n"`
	NodeId            string `json:"id"`
	UnassignedAt      string `json:"unassigned.at" json:"ua"`
	UnassignedDetails string `json:"ud" json:"unassigned.details" json:"completionSize"`
	UnassignedFor     string `json:"uf" json:"unassigned.for" json:"completionSize"`
	UnassignedReason  string `json:"ur" json:"unassigned.reason" json:"completionSize"`
	CompletionSize    string `json:"cs" json:"completion.size" json:"completionSize"`
}
