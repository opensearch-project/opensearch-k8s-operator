package requests

type ActionGroup struct {
	AllowedActions []string `json:"allowed_actions"`
	Type           string   `json:"type,omitempty"`
	Description    string   `json:"description,omitempty"`
}
