package responses

// CatPendingTasksResponse holds one row from GET /_cat/pending_tasks. A
// non-empty list means cluster-manager tasks are queued — a common signal of an
// overloaded or stuck cluster manager.
type CatPendingTasksResponse struct {
	InsertOrder string `json:"insertOrder"`
	TimeInQueue string `json:"timeInQueue"`
	Priority    string `json:"priority"`
	Source      string `json:"source"`
}
