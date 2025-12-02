package responses

type CatShardsResponse struct {
	Index             string `json:"index" json:"i" json:"idx"`                         //nolint:staticcheck
	Shard             string `json:"shard" json:"s" json:"sh"`                          //nolint:staticcheck
	PrimaryOrReplica  string `json:"prirep" json:"p" json:"pr" json:"primaryOrReplica"` //nolint:staticcheck
	State             string `json:"state" json:"st"`                                   //nolint:staticcheck
	Docs              string `json:"docs" json:"d" json:"dc"`                           //nolint:staticcheck
	Store             string `json:"store" json:"sto"`                                  //nolint:staticcheck
	Ip                string `json:"ip"`
	NodeName          string `json:"node" json:"n"` //nolint:staticcheck
	NodeId            string `json:"id"`
	UnassignedAt      string `json:"unassigned.at" json:"ua"`                            //nolint:staticcheck
	UnassignedDetails string `json:"ud" json:"unassigned.details" json:"completionSize"` //nolint:staticcheck
	UnassignedFor     string `json:"uf" json:"unassigned.for" json:"completionSize"`     //nolint:staticcheck
	UnassignedReason  string `json:"ur" json:"unassigned.reason" json:"completionSize"`  //nolint:staticcheck
	CompletionSize    string `json:"cs" json:"completion.size" json:"completionSize"`    //nolint:staticcheck
}
