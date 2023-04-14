package requests

type ActionGroup struct {
	AllowedActions []string `json:"allowed_actions,omitempty"`
	Type           string   `json:"type,omitempty"`
	Description    string   `json:"description,omitempty"`
}
