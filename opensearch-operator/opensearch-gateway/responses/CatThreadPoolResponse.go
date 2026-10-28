package responses

// CatThreadPoolResponse holds one row from GET /_cat/thread_pool. The queue and
// rejected counters are the primary signals of node overload: a growing queue
// or non-zero rejections usually means the node cannot keep up.
type CatThreadPoolResponse struct {
	NodeName string `json:"node_name"`
	Name     string `json:"name"`
	Active   string `json:"active"`
	Queue    string `json:"queue"`
	Rejected string `json:"rejected"`
}
