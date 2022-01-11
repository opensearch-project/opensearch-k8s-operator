package responses

type CatNodesResponse struct {
	Ip          string `json:"ip"`
	HeapPercent string `json:"heap.percent"`
	RamPercent  string `json:"ram.percent"`
	Cpu         string `json:"cpu"`
	Load1m      string `json:"load_1m"`
	Load5m      string `json:"load_5m"`
	Load15m     string `json:"load_15m"`
	NodeRole    string `json:"node.role"`
	Master      string `json:"master"`
	Name        string `json:"name"`
}
