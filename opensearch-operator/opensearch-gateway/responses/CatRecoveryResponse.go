package responses

// CatRecoveryResponse holds one row from GET /_cat/recovery, fetched with
// active_only=true so only in-progress recoveries are returned — letting an SRE
// tell "shards are recovering, wait" from "recovery is stuck".
type CatRecoveryResponse struct {
	Index        string `json:"index"`
	Shard        string `json:"shard"`
	Time         string `json:"time"`
	Type         string `json:"type"`
	Stage        string `json:"stage"`
	SourceNode   string `json:"source_node"`
	TargetNode   string `json:"target_node"`
	FilesPercent string `json:"files_percent"`
	BytesPercent string `json:"bytes_percent"`
}
