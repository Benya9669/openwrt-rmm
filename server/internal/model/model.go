package model

import (
	"encoding/json"
	"time"
)

type Device struct {
	ID             string          `json:"id"`
	OwnerUserID    string          `json:"owner_user_id,omitempty"`
	DNSLabel       string          `json:"dns_label,omitempty"`
	DomainName     string          `json:"domain_name,omitempty"`
	Hostname       string          `json:"hostname"`
	OpenWrtVersion string          `json:"openwrt_version"`
	LastSeenAt     *time.Time      `json:"last_seen_at"`
	CreatedAt      time.Time       `json:"created_at"`
	Inventory      json.RawMessage `json:"inventory,omitempty"`
	Metrics        json.RawMessage `json:"metrics,omitempty"`
	Online         bool            `json:"online"`
	Group          string          `json:"group,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	ActiveAlerts   int             `json:"active_alerts"`
}

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Disabled    bool      `json:"disabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type EnrollmentGrant struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	DNSLabel  string     `json:"dns_label,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Command struct {
	ID           string          `json:"id"`
	DeviceID     string          `json:"device_id"`
	Type         string          `json:"type"`
	Args         json.RawMessage `json:"args"`
	Status       string          `json:"status"`
	Result       json.RawMessage `json:"result,omitempty"`
	Output       string          `json:"output,omitempty"`
	ExitCode     *int            `json:"exit_code,omitempty"`
	AttemptCount int             `json:"attempt_count"`
	MaxAttempts  int             `json:"max_attempts"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	ClaimedAt    *time.Time      `json:"claimed_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	CancelledAt  *time.Time      `json:"cancelled_at,omitempty"`
	ExpiredAt    *time.Time      `json:"expired_at,omitempty"`
}

type AuditEvent struct {
	ID        string          `json:"id"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	DeviceID  string          `json:"device_id,omitempty"`
	CommandID string          `json:"command_id,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type MetricSample struct {
	ID        string          `json:"id"`
	DeviceID  string          `json:"device_id"`
	Inventory json.RawMessage `json:"inventory,omitempty"`
	Metrics   json.RawMessage `json:"metrics,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type Alert struct {
	ID             string          `json:"id"`
	DeviceID       string          `json:"device_id"`
	Type           string          `json:"type"`
	Severity       string          `json:"severity"`
	Status         string          `json:"status"`
	Message        string          `json:"message"`
	Details        json.RawMessage `json:"details,omitempty"`
	FirstSeenAt    time.Time       `json:"first_seen_at"`
	LastSeenAt     time.Time       `json:"last_seen_at"`
	ResolvedAt     *time.Time      `json:"resolved_at,omitempty"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string          `json:"acknowledged_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type NotificationSettings struct {
	UserID                  string     `json:"user_id,omitempty"`
	Configured              bool       `json:"configured"`
	EmailEnabled            bool       `json:"email_enabled"`
	TelegramEnabled         bool       `json:"telegram_enabled"`
	TelegramChatID          string     `json:"telegram_chat_id,omitempty"`
	NotifyWarning           bool       `json:"notify_warning"`
	NotifyCritical          bool       `json:"notify_critical"`
	NotifyResolved          bool       `json:"notify_resolved"`
	MemoryThresholdPercent  int        `json:"memory_threshold_percent"`
	DiskThresholdPercent    int        `json:"disk_threshold_percent"`
	PacketLossPercent       int        `json:"packet_loss_percent"`
	LatencyThresholdMS      int        `json:"latency_threshold_ms"`
	RepeatMinutes           int        `json:"repeat_minutes"`
	Timezone                string     `json:"timezone"`
	QuietHoursEnabled       bool       `json:"quiet_hours_enabled"`
	QuietHoursStart         string     `json:"quiet_hours_start"`
	QuietHoursEnd           string     `json:"quiet_hours_end"`
	AlertsPausedUntil       *time.Time `json:"alerts_paused_until,omitempty"`
	WebhookEnabled          bool       `json:"webhook_enabled"`
	WebhookURL              string     `json:"webhook_url,omitempty"`
	WebhookSecret           string     `json:"-"`
	WebhookSecretConfigured bool       `json:"webhook_secret_configured"`
	CreatedAt               time.Time  `json:"created_at,omitempty"`
	UpdatedAt               time.Time  `json:"updated_at,omitempty"`
}

type DeviceNotificationSettings struct {
	DeviceID       string     `json:"device_id"`
	Enabled        bool       `json:"enabled"`
	NotifyWarning  bool       `json:"notify_warning"`
	NotifyCritical bool       `json:"notify_critical"`
	NotifyResolved bool       `json:"notify_resolved"`
	PausedUntil    *time.Time `json:"paused_until,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
}

type InboxNotification struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	DeviceID   string     `json:"device_id,omitempty"`
	IncidentID string     `json:"incident_id,omitempty"`
	Severity   string     `json:"severity"`
	Event      string     `json:"event"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type LANClient struct {
	DeviceID      string     `json:"device_id,omitempty"`
	Key           string     `json:"key"`
	MAC           string     `json:"mac,omitempty"`
	IP            string     `json:"ip,omitempty"`
	Hostname      string     `json:"hostname,omitempty"`
	Interface     string     `json:"interface,omitempty"`
	Connection    string     `json:"connection"`
	Status        string     `json:"status"`
	Confirmation  string     `json:"confirmation,omitempty"`
	FirstSeenAt   time.Time  `json:"first_seen_at"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	LastCheckedAt time.Time  `json:"last_checked_at"`
}

type NotificationDelivery struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id,omitempty"`
	DeviceID          string     `json:"device_id,omitempty"`
	AlertID           string     `json:"alert_id,omitempty"`
	Event             string     `json:"event"`
	Channel           string     `json:"channel"`
	Status            string     `json:"status"`
	Title             string     `json:"title"`
	Body              string     `json:"body"`
	Destination       string     `json:"-"`
	DestinationMasked string     `json:"destination"`
	Error             string     `json:"error,omitempty"`
	AttemptCount      int        `json:"attempt_count"`
	MaxAttempts       int        `json:"max_attempts"`
	CreatedAt         time.Time  `json:"created_at"`
	LastAttemptAt     *time.Time `json:"last_attempt_at,omitempty"`
	NextAttemptAt     *time.Time `json:"next_attempt_at,omitempty"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
}

type RemoteSession struct {
	ID          string     `json:"id"`
	DeviceID    string     `json:"device_id"`
	Target      string     `json:"target"`
	Status      string     `json:"status"`
	ServerHost  string     `json:"server_host,omitempty"`
	ServerPort  int        `json:"server_port,omitempty"`
	RemotePort  int        `json:"remote_port,omitempty"`
	LuCIPort    int        `json:"luci_port,omitempty"`
	LuCIScheme  string     `json:"luci_scheme,omitempty"`
	LocalHost   string     `json:"local_host,omitempty"`
	LocalPort   int        `json:"local_port,omitempty"`
	CommandID   string     `json:"command_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	AccessState string     `json:"access_state,omitempty"`
}
