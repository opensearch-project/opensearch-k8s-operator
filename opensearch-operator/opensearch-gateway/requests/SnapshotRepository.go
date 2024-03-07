package requests

type SnapshotRepository struct {
	Type     string            `json:"type"`
	Settings map[string]string `json:"settings,omitempty"`
}
