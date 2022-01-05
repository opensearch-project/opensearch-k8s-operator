package responses

type NodeStatResponse struct {
	Id                string
	Name              string                        `json:"name"`
	Timestamp         uint32                        `json:"timestamp"`
	TransportAddress  string                        `json:"transport_address"`
	Host              string                        `json:"host"`
	Ip                string                        `json:"ip"`
	Roles             []string                      `json:"roles"`
	Attributes        map[string]string             `json:"attributes"`
	Indices           map[string]interface{}        `json:"indices"`
	Os                map[string]interface{}        `json:"os"`
	Process           map[string]interface{}        `json:"process"`
	Jvm               map[string]interface{}        `json:"jvm"`
	ThreadPool        map[string]NodeStatThreadPool `json:"thread_pool"`
	Fs                map[string]interface{}        `json:"Fs"`
	Transport         map[string]interface{}        `json:"transport"`
	Http              map[string]interface{}        `json:"http"`
	Breakers          map[string]NodeStatBreakers   `json:"breakers"`
	Script            map[string]interface{}        `json:"script"`
	Discovery         map[string]interface{}        `json:"discovery"`
	Ingest            map[string]interface{}        `json:"ingest"`
	AdaptiveSelection map[string]interface{}        `json:"adaptive_selection"`
	ScriptCache       map[string]interface{}        `json:"script_cache"`
	IndexingPressure  map[string]interface{}        `json:"indexing_pressure"`
}

type NodeStatThreadPool struct {
	Threads   uint16 `json:"threads"`
	Queue     uint16 `json:"queue"`
	Active    uint16 `json:"active"`
	Rejected  uint16 `json:"rejected"`
	Largest   uint16 `json:"largest"`
	Completed uint16 `json:"completed"`
}

type NodeStatBreakers struct {
	LimitSizeInBytes     uint32 `json:"limit_size_in_bytes"`
	EstimatedSizeInBytes uint32 `json:"estimated_size_in_bytes"`
	Overhead             uint16 `json:"overhead"`
	Tripped              uint16 `json:"tripped"`
}

type NodeStatAdaptiveSelection struct {
	OutgoingSearches  uint16 `json:"outgoing_searches"`
	AvgQueueSize      uint16 `json:"avg_queue_size"`
	AvgServiceTimeNs  uint16 `json:"avg_service_time_ns"`
	AvgResponseTimeNs uint16 `json:"avg_response_time_ns"`
	Rank              string `json:"rank"`
}

type NodeStatScriptCache struct {
	Sum      map[string]interface{}  `json:"sum"`
	Contexts []NodeStatScriptContext `json:"contexts"`
}

type NodeStatScriptContext struct {
	Context                   string `json:"context"`
	Compilations              uint16 `json:"compilations"`
	CacheEvictions            uint16 `json:"cache_evictions"`
	CompilationLimitTriggered uint16 `json:"compilation_limit_triggered"`
}
