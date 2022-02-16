package responses

type CatHealthResponse struct {
	Cluster             string `json:"cluster"`
	Status              string `json:"status"`
	NodeTotal           string `json:"node.total"`
	NodeData            string `json:"node.data"`
	Shards              string `json:"shards"`
	PrimaryShards       string `json:"pri"`
	ReloadingShards     string `json:"relo"`
	InitializingShards  string `json:"init"`
	UnAssignShards      string `json:"unassign"`
	PendingTasks        string `json:"pending_tasks"`
	ActiveShardsPercent string `json:"active_shards_percent"`
	MaxTaskWaitTime     string `json:"max_task_wait_time"`
}
