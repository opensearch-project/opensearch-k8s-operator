package opensearchoperator

import "github.com/projectcapsule/capsule/api/v1beta2/NotificationChannel"

// NotificationChannel represents a notification channel for Matrix events.
type NotificationChannel struct {
    ID     string `json:"id"`
    Type   string `json:"type"`
    Channel string `json:"channel"`
    MessageTemplate string `json:"message_template"`
}

// NotificationChannel is the gateway struct type that holds notification channels.
type NotificationChannel struct {
    ID     string `json:"id"`
    Type   string `json:"type"`
    Channel string `json:"channel"`
    MessageTemplate string `json:"message_template"`
}
