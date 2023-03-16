package responses

type NodesStatsResponse struct {
	GeneralInfo NodesStatsGeneralInfoResponse `json:"_nodes"`
	ClusterName string                        `json:"cluster_name"`
	Nodes       map[string]NodeStatResponse   `json:"nodes"`
}

type NodesStatsGeneralInfoResponse struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

type NodeStatResponse struct {
	Id                string
	Name              string                               `json:"name"`
	Timestamp         uint64                               `json:"timestamp"`
	TransportAddress  string                               `json:"transport_address"`
	Host              string                               `json:"host"`
	Ip                string                               `json:"ip"`
	Roles             []string                             `json:"roles"`
	Attributes        map[string]string                    `json:"attributes"`
	Indices           map[string]interface{}               `json:"indices"`
	Os                map[string]interface{}               `json:"os"`
	Process           map[string]interface{}               `json:"process"`
	Jvm               map[string]interface{}               `json:"jvm"`
	ThreadPool        map[string]NodeStatThreadPool        `json:"thread_pool"`
	Fs                map[string]interface{}               `json:"fs"`
	Transport         map[string]interface{}               `json:"transport"`
	Http              map[string]interface{}               `json:"http"`
	Breakers          map[string]NodeStatBreakers          `json:"breakers"`
	Script            map[string]interface{}               `json:"script"`
	Discovery         map[string]interface{}               `json:"discovery"`
	Ingest            map[string]interface{}               `json:"ingest"`
	AdaptiveSelection map[string]NodeStatAdaptiveSelection `json:"adaptive_selection"`
	ScriptCache       NodeStatScriptCache                  `json:"script_cache"`
	IndexingPressure  map[string]interface{}               `json:"indexing_pressure"`
}

type NodeStatThreadPool struct {
	Threads   uint32 `json:"threads"`
	Queue     uint32 `json:"queue"`
	Active    uint32 `json:"active"`
	Rejected  uint32 `json:"rejected"`
	Largest   uint32 `json:"largest"`
	Completed uint32 `json:"completed"`
}

type NodeStatBreakers struct {
	LimitSizeInBytes     uint64  `json:"limit_size_in_bytes"`
	EstimatedSizeInBytes uint64  `json:"estimated_size_in_bytes"`
	Overhead             float32 `json:"overhead"`
	Tripped              uint32  `json:"tripped"`
}

type NodeStatAdaptiveSelection struct {
	OutgoingSearches  uint32 `json:"outgoing_searches"`
	AvgQueueSize      uint32 `json:"avg_queue_size"`
	AvgServiceTimeNs  uint32 `json:"avg_service_time_ns"`
	AvgResponseTimeNs uint32 `json:"avg_response_time_ns"`
	Rank              string `json:"rank"`
}

type NodeStatScriptCache struct {
	Sum      map[string]interface{}  `json:"sum"`
	Contexts []NodeStatScriptContext `json:"contexts"`
}

type NodeStatScriptContext struct {
	Context                   string `json:"context"`
	Compilations              uint32 `json:"compilations"`
	CacheEvictions            uint32 `json:"cache_evictions"`
	CompilationLimitTriggered uint32 `json:"compilation_limit_triggered"`
}
