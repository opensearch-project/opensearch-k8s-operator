package requests

type ReRouteMoveAction struct {
	Index    string `json:"index"`
	Shard    string `json:"shard"`
	FromNode string `json:"from_node"`
	ToNode   string `json:"to_node"`
}
