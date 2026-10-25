package opensearchoperator

import (
	"testing"
)

func TestNotificationChannel(t *testing.T) {
	tests := []struct {
		name     string
		wantsID  string
		wantsType string
		wantsChannel string
		wantsTemplate string
	}{
		{
			name:     "id only",
			wantsID:  "test-id",
			wantsType: "matrix",
			wantsChannel: "matrix.ingest",
			wantsTemplate: "Test notification",
		},
		{
			name:     "full struct",
			wantsID:  "full-id",
			wantsType: "matrix",
			wantsChannel: "matrix.ingest",
			wantsTemplate: "Full notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := NotificationChannel{
				ID:     tt.wantsID,
				Type:   tt.wantsType,
				Channel: tt.wantsChannel,
				MessageTemplate: tt.wantsTemplate,
			}
			// Verify all fields are set correctly
			if channel.ID != tt.wantsID {
				t.Errorf("ID mismatch: got %s, want %s", channel.ID, tt.wantsID)
			}
			if channel.Type != tt.wantsType {
				t.Errorf("Type mismatch: got %s, want %s", channel.Type, tt.wantsType)
			}
			if channel.Channel != tt.wantsChannel {
				t.Errorf("Channel mismatch: got %s, want %s", channel.Channel, tt.wantsChannel)
			}
			if channel.MessageTemplate != tt.wantsTemplate {
				t.Errorf("MessageTemplate mismatch: got %q, want %q", channel.MessageTemplate, tt.wantsTemplate)
			}
		}
	}
}
