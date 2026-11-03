package responses

type CatSegmentsResponse struct {
	Index            string `json:"index"`
	Shard            string `json:"shard"`
	PrimaryOrReplica string `json:"prirep"`
	Ip               string `json:"ip"`
	Segment          string `json:"segment"`
	Generation       string `json:"generation"`
	DocsCount        string `json:"docs.count"`
	DocsDeleted      string `json:"docs.deleted"`
	Size             string `json:"size"`
	SizeMemory       string `json:"size.memory"`
	Committed        string `json:"committed"`
	Searchable       string `json:"searchable"`
	Version          string `json:"version"`
	Compound         string `json:"compound"`
}
