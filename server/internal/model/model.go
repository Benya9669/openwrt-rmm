package model

import (
	"encoding/json"
	"time"
)

type Device struct {
	ID             string          `json:"id"`
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

type RemoteSession struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"device_id"`
	Target     string     `json:"target"`
	Status     string     `json:"status"`
	ServerHost string     `json:"server_host,omitempty"`
	ServerPort int        `json:"server_port,omitempty"`
	RemotePort int        `json:"remote_port,omitempty"`
	LocalHost  string     `json:"local_host,omitempty"`
	LocalPort  int        `json:"local_port,omitempty"`
	CommandID  string     `json:"command_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
}
