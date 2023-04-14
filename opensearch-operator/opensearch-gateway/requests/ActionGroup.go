package requests

type ActionGroup struct {
	AllowedActions []string `json:"allowedActions,omitempty"`
	Type           string   `json:"type,omitempty"`
	Description    string   `json:"description,omitempty"`
}
