package requests

// SnapshotPolicy is the root object that wraps the snapshot policy specification.
type SnapshotPolicy struct {
	Policy SnapshotPolicySpec `json:"policy"` // Contains all settings related to the snapshot policy.
}

// SnapshotPolicySpec defines the configuration of a snapshot lifecycle policy.
type SnapshotPolicySpec struct {
	PolicyName     string                `json:"name"`                   // Unique name of the snapshot policy.
	Description    *string               `json:"description,omitempty"`  // Optional human-readable description.
	Enabled        *bool                 `json:"enabled,omitempty"`      // Whether the policy is enabled or not.
	SnapshotConfig SnapshotConfig        `json:"snapshot_config"`        // Configuration of the snapshot itself.
	Creation       SnapshotCreation      `json:"creation"`               // Defines when and how snapshots are created.
	Deletion       *SnapshotDeletion     `json:"deletion,omitempty"`     // Optional settings for automatic snapshot deletion.
	Notification   *SnapshotNotification `json:"notification,omitempty"` // Optional settings for notification triggers.
}

// SnapshotConfig defines the snapshot creation parameters.
type SnapshotConfig struct {
	DateFormat         string            `json:"date_format,omitempty"`          // Format for timestamp in snapshot name (e.g. yyyy-MM-dd).
	DateFormatTimezone string            `json:"date_format_timezone,omitempty"` // Timezone for date_format (e.g. UTC, PST).
	Indices            string            `json:"indices,omitempty"`              // Indices to include in the snapshot, supports wildcards.
	Repository         string            `json:"repository"`                     // Repository name where snapshots are stored.
	IgnoreUnavailable  bool              `json:"ignore_unavailable,omitempty"`   // Whether to ignore unavailable indices.
	IncludeGlobalState bool              `json:"include_global_state,omitempty"` // Whether to include global cluster state in the snapshot.
	Partial            bool              `json:"partial,omitempty"`              // Whether to allow partial snapshots.
	Metadata           map[string]string `json:"metadata,omitempty"`             // Optional custom metadata to associate with the snapshot.
}

// SnapshotCreation defines scheduling and timeout settings for snapshot creation.
type SnapshotCreation struct {
	Schedule  CronSchedule `json:"schedule"`             // Cron-based schedule for automatic snapshot creation.
	TimeLimit *string      `json:"time_limit,omitempty"` // Maximum allowed time for snapshot creation (e.g., "30m").
}

// CronSchedule wraps a cron expression for scheduling.
type CronSchedule struct {
	Cron CronExpression `json:"cron"` // Standard cron expression with timezone support.
}

// CronExpression represents the cron schedule expression and associated timezone.
type CronExpression struct {
	Expression string `json:"expression"` // Cron expression (e.g. "0 0 * * *").
	Timezone   string `json:"timezone"`   // Timezone in which the schedule should run (e.g., "UTC").
}

// SnapshotDeletion configures automatic snapshot deletion.
type SnapshotDeletion struct {
	Schedule        *CronSchedule            `json:"schedule,omitempty"`   // Optional cron schedule for deletion checks.
	TimeLimit       *string                  `json:"time_limit,omitempty"` // Optional max time allowed for deletion process.
	DeleteCondition *SnapshotDeleteCondition `json:"condition,omitempty"`  // Conditions that determine which snapshots to delete.
}

// SnapshotDeleteCondition specifies thresholds for snapshot deletion.
type SnapshotDeleteCondition struct {
	MaxCount *int    `json:"max_count,omitempty"` // Delete oldest snapshots if count exceeds this number.
	MaxAge   *string `json:"max_age,omitempty"`   // Delete snapshots older than this duration (e.g., "30d").
	MinCount *int    `json:"min_count,omitempty"` // Always retain at least this many snapshots.
}

// SnapshotNotification defines where and when notifications should be sent.
type SnapshotNotification struct {
	Channel    NotificationChannel     `json:"channel"`              // Notification channel (e.g., Slack, Webhook).
	Conditions *NotificationConditions `json:"conditions,omitempty"` // Conditions under which to trigger notifications.
}

// NotificationChannel represents a notification channel.
type NotificationChannel struct {
	ID string `json:"id"` // Unique identifier of the channel.
}

// NotificationConditions specifies events that should trigger notifications.
type NotificationConditions struct {
	Creation *bool `json:"creation,omitempty"` // Notify when snapshot is created.
	Deletion *bool `json:"deletion,omitempty"` // Notify when snapshot is deleted.
	Failure  *bool `json:"failure,omitempty"`  // Notify on snapshot failure.
}
