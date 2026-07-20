package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type CommandListOptions struct {
	Limit  int
	Offset int
}

type MetricHistoryOptions struct {
	Limit int
}

type AuditListOptions struct {
	DeviceID string
	Limit    int
	Offset   int
}

type AlertListOptions struct {
	DeviceID string
	Status   string
	Limit    int
}

type PurgeOptions struct {
	DeviceID string
}

type RemoteSessionListOptions struct {
	Limit int
}

type EnrolledDevice struct {
	DeviceID    string
	DeviceToken string
}

func OpenSQLite(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// SQLite permits one writer at a time. Keep access serialized inside this
	// process and wait briefly for locks held during startup or maintenance.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, stmt := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	s := &Store{db: db}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS devices (
	id TEXT PRIMARY KEY,
	token TEXT NOT NULL UNIQUE,
	token_hash TEXT NOT NULL DEFAULT '',
	owner_user_id TEXT NOT NULL DEFAULT '',
	dns_label TEXT NOT NULL DEFAULT '',
	hostname TEXT NOT NULL,
	openwrt_version TEXT NOT NULL,
	inventory_json TEXT NOT NULL DEFAULT '{}',
	metrics_json TEXT NOT NULL DEFAULT '{}',
	group_name TEXT NOT NULL DEFAULT '',
	tags_json TEXT NOT NULL DEFAULT '[]',
	last_seen_at TEXT,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS commands (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	type TEXT NOT NULL,
	args_json TEXT NOT NULL DEFAULT '{}',
	status TEXT NOT NULL,
	result_json TEXT NOT NULL DEFAULT '{}',
	output TEXT NOT NULL DEFAULT '',
		exit_code INTEGER,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		created_at TEXT NOT NULL,
		expires_at TEXT,
		claimed_at TEXT,
		completed_at TEXT,
		cancelled_at TEXT,
		expired_at TEXT,
		FOREIGN KEY(device_id) REFERENCES devices(id)
);

CREATE INDEX IF NOT EXISTS idx_commands_device_status ON commands(device_id, status);
CREATE INDEX IF NOT EXISTS idx_commands_device_created_at ON commands(device_id, created_at);

CREATE TABLE IF NOT EXISTS metric_samples (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	inventory_json TEXT NOT NULL DEFAULT '{}',
	metrics_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	FOREIGN KEY(device_id) REFERENCES devices(id)
);

CREATE INDEX IF NOT EXISTS idx_metric_samples_device_created_at ON metric_samples(device_id, created_at);

CREATE TABLE IF NOT EXISTS alerts (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	type TEXT NOT NULL,
	severity TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL,
	details_json TEXT NOT NULL DEFAULT '{}',
	first_seen_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	resolved_at TEXT,
	acknowledged_at TEXT,
	acknowledged_by TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(device_id) REFERENCES devices(id)
);

CREATE INDEX IF NOT EXISTS idx_alerts_device_status ON alerts(device_id, status);
CREATE INDEX IF NOT EXISTS idx_alerts_status_last_seen ON alerts(status, last_seen_at);

CREATE TABLE IF NOT EXISTS remote_sessions (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	target TEXT NOT NULL,
	status TEXT NOT NULL,
	server_host TEXT NOT NULL DEFAULT '',
	server_port INTEGER NOT NULL DEFAULT 22,
	remote_port INTEGER NOT NULL DEFAULT 0,
	luci_port INTEGER NOT NULL DEFAULT 0,
	luci_scheme TEXT NOT NULL DEFAULT 'http',
	local_host TEXT NOT NULL DEFAULT '127.0.0.1',
	local_port INTEGER NOT NULL DEFAULT 22,
	command_id TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	started_at TEXT,
	closed_at TEXT,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(device_id) REFERENCES devices(id)
);

CREATE TABLE IF NOT EXISTS audit_events (
	id TEXT PRIMARY KEY,
	actor TEXT NOT NULL,
	action TEXT NOT NULL,
	device_id TEXT NOT NULL DEFAULT '',
	command_id TEXT NOT NULL DEFAULT '',
	details_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_events_device_created_at ON audit_events(device_id, created_at);

CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL COLLATE NOCASE UNIQUE,
	display_name TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL CHECK(role IN ('admin', 'user')),
	disabled INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS operator_sessions (
	token_hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS enrollment_grants (
	id TEXT PRIMARY KEY,
	token_hash TEXT NOT NULL UNIQUE,
	user_id TEXT NOT NULL,
	dns_label TEXT NOT NULL DEFAULT '',
	expires_at TEXT NOT NULL,
	used_at TEXT,
	created_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS device_access_grants (
	token_hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	device_id TEXT NOT NULL,
	remote_session_id TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(device_id) REFERENCES devices(id),
	FOREIGN KEY(remote_session_id) REFERENCES remote_sessions(id)
);

CREATE TABLE IF NOT EXISTS device_access_sessions (
	token_hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	device_id TEXT NOT NULL,
	remote_session_id TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(device_id) REFERENCES devices(id),
	FOREIGN KEY(remote_session_id) REFERENCES remote_sessions(id)
);
`)
	if err != nil {
		return err
	}

	for _, stmt := range []string{
		`ALTER TABLE devices ADD COLUMN group_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE devices ADD COLUMN token_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN dns_label TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE commands ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE commands ADD COLUMN max_attempts INTEGER NOT NULL DEFAULT 3`,
		`ALTER TABLE commands ADD COLUMN expires_at TEXT`,
		`ALTER TABLE commands ADD COLUMN cancelled_at TEXT`,
		`ALTER TABLE commands ADD COLUMN expired_at TEXT`,
		`ALTER TABLE remote_sessions ADD COLUMN target TEXT NOT NULL DEFAULT 'ssh'`,
		`ALTER TABLE remote_sessions ADD COLUMN status TEXT NOT NULL DEFAULT 'failed'`,
		`ALTER TABLE remote_sessions ADD COLUMN server_host TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE remote_sessions ADD COLUMN server_port INTEGER NOT NULL DEFAULT 22`,
		`ALTER TABLE remote_sessions ADD COLUMN remote_port INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE remote_sessions ADD COLUMN luci_port INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE remote_sessions ADD COLUMN luci_scheme TEXT NOT NULL DEFAULT 'http'`,
		`ALTER TABLE remote_sessions ADD COLUMN local_host TEXT NOT NULL DEFAULT '127.0.0.1'`,
		`ALTER TABLE remote_sessions ADD COLUMN local_port INTEGER NOT NULL DEFAULT 22`,
		`ALTER TABLE remote_sessions ADD COLUMN command_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE remote_sessions ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE remote_sessions ADD COLUMN expires_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE remote_sessions ADD COLUMN started_at TEXT`,
		`ALTER TABLE remote_sessions ADD COLUMN closed_at TEXT`,
		`ALTER TABLE remote_sessions ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}

	for _, stmt := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_dns_label ON devices(dns_label) WHERE dns_label != ''`,
		`CREATE INDEX IF NOT EXISTS idx_devices_owner_created_at ON devices(owner_user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_operator_sessions_user_expires ON operator_sessions(user_id, expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollment_grants_user_created ON enrollment_grants(user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_device_access_sessions_device_expires ON device_access_sessions(device_id, expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_remote_sessions_device_created_at ON remote_sessions(device_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_remote_sessions_status_expires_at ON remote_sessions(status, expires_at)`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return s.migrateDeviceTokens(ctx)
}

func (s *Store) EnrollDevice(ctx context.Context, hostname, openwrtVersion string) (EnrolledDevice, error) {
	id, err := randomID("dev")
	if err != nil {
		return EnrolledDevice{}, err
	}
	token, err := randomID("tok")
	if err != nil {
		return EnrolledDevice{}, err
	}

	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown"
	}
	openwrtVersion = strings.TrimSpace(openwrtVersion)
	if openwrtVersion == "" {
		openwrtVersion = "unknown"
	}
	ownerUserID := ""
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE role = 'admin' AND disabled = 0 ORDER BY created_at LIMIT 1`).Scan(&ownerUserID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return EnrolledDevice{}, err
	}
	dnsLabel := ""
	if ownerUserID != "" {
		dnsLabel = generatedDNSLabel(hostname, id)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO devices (id, token, token_hash, owner_user_id, dns_label, hostname, openwrt_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, id, "redacted:"+id, TokenHash(token), ownerUserID, dnsLabel, hostname, openwrtVersion, nowText())
	if err != nil {
		return EnrolledDevice{}, err
	}

	return EnrolledDevice{DeviceID: id, DeviceToken: token}, nil
}

func (s *Store) AuthorizeDevice(ctx context.Context, deviceID, token string) (bool, error) {
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(token) == "" {
		return false, nil
	}

	var exists bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(SELECT 1 FROM devices WHERE id = ? AND token_hash = ?)
`, deviceID, TokenHash(token)).Scan(&exists)
	return exists, err
}

func (s *Store) SaveHeartbeat(ctx context.Context, deviceID string, inventory, metrics json.RawMessage) ([]model.Command, error) {
	if err := s.ExpireClaimedCommands(ctx, 2*time.Minute); err != nil {
		return nil, err
	}

	inventory = NormalizeRawJSON(inventory)
	metrics = NormalizeRawJSON(metrics)
	now := nowText()

	_, err := s.db.ExecContext(ctx, `
UPDATE devices
SET inventory_json = ?, metrics_json = ?, last_seen_at = ?
WHERE id = ?
`, string(inventory), string(metrics), now, deviceID)
	if err != nil {
		return nil, err
	}
	if err := s.AddMetricSample(ctx, deviceID, inventory, metrics, now); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, device_id, type, args_json, status, result_json, output, exit_code, attempt_count, max_attempts, created_at, expires_at, claimed_at, completed_at, cancelled_at, expired_at
FROM commands
WHERE device_id = ? AND status = 'queued' AND (expires_at IS NULL OR julianday(expires_at) > julianday(?))
ORDER BY created_at ASC
LIMIT 5
`, deviceID, nowText())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commands := make([]model.Command, 0)
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		commands = append(commands, c)
	}
	return commands, rows.Err()
}

func (s *Store) ClaimNextCommand(ctx context.Context, deviceID string) (model.Command, bool, error) {
	if err := s.ExpireClaimedCommands(ctx, 2*time.Minute); err != nil {
		return model.Command{}, false, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Command{}, false, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
SELECT id, device_id, type, args_json, status, result_json, output, exit_code, attempt_count, max_attempts, created_at, expires_at, claimed_at, completed_at, cancelled_at, expired_at
FROM commands
WHERE device_id = ? AND status = 'queued' AND attempt_count < max_attempts AND (expires_at IS NULL OR julianday(expires_at) > julianday(?))
ORDER BY created_at ASC
LIMIT 1
`, deviceID, nowText())

	c, err := scanCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Command{}, false, nil
	} else if err != nil {
		return model.Command{}, false, err
	}

	res, err := tx.ExecContext(ctx, `
UPDATE commands
SET status = 'claimed', claimed_at = ?, attempt_count = attempt_count + 1
WHERE id = ? AND status = 'queued'
`, nowText(), c.ID)
	if err != nil {
		return model.Command{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.Command{}, false, nil
	}
	if err := tx.Commit(); err != nil {
		return model.Command{}, false, err
	}
	c.Status = "claimed"
	c.AttemptCount++
	return c, true, nil
}

func (s *Store) SaveCommandResult(ctx context.Context, commandID, deviceID, status string, exitCode int, output string, result json.RawMessage) (bool, error) {
	result = NormalizeRawJSON(result)
	output = RedactSensitiveOutput(output)
	res, err := s.db.ExecContext(ctx, `
UPDATE commands
SET status = ?, result_json = ?, output = ?, exit_code = ?, completed_at = ?
WHERE id = ? AND device_id = ?
AND status != 'cancelled'
AND status != 'expired'
`, status, string(result), output, exitCode, nowText(), commandID, deviceID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		if err := s.updateRemoteSessionFromCommandResult(ctx, commandID, status); err != nil {
			return false, err
		}
	}
	return n > 0, nil
}

func (s *Store) updateRemoteSessionFromCommandResult(ctx context.Context, commandID, status string) error {
	now := nowText()
	switch status {
	case "completed":
		_, err := s.db.ExecContext(ctx, `
UPDATE remote_sessions
SET status = 'active', started_at = COALESCE(started_at, ?), updated_at = ?
WHERE command_id = ? AND status IN ('requested', 'queued')
`, now, now, commandID)
		return err
	case "failed":
		_, err := s.db.ExecContext(ctx, `
UPDATE remote_sessions
SET status = 'failed', closed_at = COALESCE(closed_at, ?), updated_at = ?
WHERE command_id = ? AND status IN ('requested', 'queued', 'active')
`, now, now, commandID)
		return err
	default:
		return nil
	}
}

func (s *Store) ExpireClaimedCommands(ctx context.Context, maxAge time.Duration) error {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339Nano)
	now := nowText()
	if _, err := s.db.ExecContext(ctx, `
UPDATE commands
SET status = 'expired', expired_at = ?
WHERE status IN ('queued', 'claimed') AND expires_at IS NOT NULL AND julianday(expires_at) <= julianday(?)
`, now, now); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE commands
SET status = 'expired', expired_at = ?
WHERE status = 'claimed' AND claimed_at IS NOT NULL AND julianday(claimed_at) <= julianday(?) AND attempt_count >= max_attempts
`, now, cutoff); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE commands
SET status = 'queued', claimed_at = NULL
WHERE status = 'claimed' AND claimed_at IS NOT NULL AND julianday(claimed_at) <= julianday(?) AND attempt_count < max_attempts
`, cutoff)
	return err
}

func (s *Store) ListDevices(ctx context.Context) ([]model.Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT d.id, d.owner_user_id, d.dns_label, d.hostname, d.openwrt_version, d.last_seen_at, d.created_at, d.inventory_json, d.metrics_json, d.group_name, d.tags_json,
	(SELECT COUNT(1) FROM alerts a WHERE a.device_id = d.id AND a.status IN ('active', 'acknowledged'))
FROM devices d
ORDER BY d.created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]model.Device, 0)
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) GetDevice(ctx context.Context, deviceID string) (model.Device, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT d.id, d.owner_user_id, d.dns_label, d.hostname, d.openwrt_version, d.last_seen_at, d.created_at, d.inventory_json, d.metrics_json, d.group_name, d.tags_json,
	(SELECT COUNT(1) FROM alerts a WHERE a.device_id = d.id AND a.status IN ('active', 'acknowledged'))
FROM devices d
WHERE d.id = ?
`, deviceID)

	d, err := scanDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Device{}, false, nil
	}
	if err != nil {
		return model.Device{}, false, err
	}
	return d, true, nil
}

func (s *Store) UpdateDeviceFleet(ctx context.Context, deviceID, group string, tags []string) (model.Device, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return model.Device{}, false, err
	}
	if !exists {
		return model.Device{}, false, nil
	}
	group = strings.TrimSpace(group)
	tags = normalizeTags(tags)
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return model.Device{}, false, err
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE devices
SET group_name = ?, tags_json = ?
WHERE id = ?
`, group, string(tagsJSON), deviceID); err != nil {
		return model.Device{}, false, err
	}
	d, found, err := s.GetDevice(ctx, deviceID)
	return d, found, err
}

func (s *Store) DeleteDevice(ctx context.Context, deviceID string) (bool, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false, nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM device_access_sessions WHERE device_id = ?`,
		`DELETE FROM device_access_grants WHERE device_id = ?`,
		`DELETE FROM remote_sessions WHERE device_id = ?`,
		`DELETE FROM commands WHERE device_id = ?`,
		`DELETE FROM metric_samples WHERE device_id = ?`,
		`DELETE FROM alerts WHERE device_id = ?`,
		`DELETE FROM audit_events WHERE device_id = ?`,
		`DELETE FROM devices WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, deviceID); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *Store) AddMetricSample(ctx context.Context, deviceID string, inventory, metrics json.RawMessage, createdAt string) error {
	id, err := randomID("met")
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO metric_samples (id, device_id, inventory_json, metrics_json, created_at)
VALUES (?, ?, ?, ?, ?)
`, id, deviceID, string(inventory), string(metrics), createdAt)
	return err
}

func (s *Store) ListMetricSamples(ctx context.Context, deviceID string, opts MetricHistoryOptions) ([]model.MetricSample, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	limit := opts.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, device_id, inventory_json, metrics_json, created_at
FROM metric_samples
WHERE device_id = ?
ORDER BY created_at DESC
LIMIT ?
`, deviceID, limit)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	samples := make([]model.MetricSample, 0)
	for rows.Next() {
		var sample model.MetricSample
		var inventory string
		var metrics string
		var createdAt string
		if err := rows.Scan(&sample.ID, &sample.DeviceID, &inventory, &metrics, &createdAt); err != nil {
			return nil, false, err
		}
		sample.Inventory = json.RawMessage(inventory)
		sample.Metrics = json.RawMessage(metrics)
		sample.CreatedAt = parseTime(createdAt)
		samples = append(samples, sample)
	}
	return samples, true, rows.Err()
}

func (s *Store) SyncDeviceAlerts(ctx context.Context, deviceID string, active []model.Alert) ([]model.Alert, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}

	now := nowText()
	activeIDs := make([]string, 0, len(active))
	for _, alert := range active {
		activeIDs = append(activeIDs, alert.ID)
		details := NormalizeRawJSON(alert.Details)
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO alerts (id, device_id, type, severity, status, message, details_json, first_seen_at, last_seen_at, created_at, updated_at)
VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	severity = excluded.severity,
	status = CASE WHEN alerts.status = 'acknowledged' THEN 'acknowledged' ELSE 'active' END,
	message = excluded.message,
	details_json = excluded.details_json,
	last_seen_at = excluded.last_seen_at,
	resolved_at = NULL,
	updated_at = excluded.updated_at
`, alert.ID, deviceID, alert.Type, alert.Severity, alert.Message, string(details), now, now, now, now); err != nil {
			return nil, true, err
		}
	}

	if len(activeIDs) == 0 {
		if _, err := s.db.ExecContext(ctx, `
UPDATE alerts
SET status = 'resolved', resolved_at = ?, updated_at = ?
WHERE device_id = ? AND status IN ('active', 'acknowledged')
`, now, now, deviceID); err != nil {
			return nil, true, err
		}
	} else {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(activeIDs)), ",")
		args := []any{now, now, deviceID}
		for _, id := range activeIDs {
			args = append(args, id)
		}
		if _, err := s.db.ExecContext(ctx, `
UPDATE alerts
SET status = 'resolved', resolved_at = ?, updated_at = ?
WHERE device_id = ? AND status IN ('active', 'acknowledged') AND id NOT IN (`+placeholders+`)
`, args...); err != nil {
			return nil, true, err
		}
	}

	alerts, err := s.ListAlerts(ctx, AlertListOptions{DeviceID: deviceID, Status: "open", Limit: 100})
	return alerts, true, err
}

func (s *Store) ListAlerts(ctx context.Context, opts AlertListOptions) ([]model.Alert, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = "open"
	}

	query := `
SELECT id, device_id, type, severity, status, message, details_json, first_seen_at, last_seen_at, resolved_at, acknowledged_at, acknowledged_by, created_at
FROM alerts
`
	args := []any{}
	where := []string{}
	if strings.TrimSpace(opts.DeviceID) != "" {
		where = append(where, "device_id = ?")
		args = append(args, opts.DeviceID)
	}
	if status == "open" {
		where = append(where, "status IN ('active', 'acknowledged')")
	} else if status != "all" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if len(where) > 0 {
		query += "WHERE " + strings.Join(where, " AND ") + "\n"
	}
	query += `ORDER BY last_seen_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]model.Alert, 0)
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func (s *Store) AcknowledgeAlert(ctx context.Context, deviceID, alertID, actor string) (model.Alert, bool, error) {
	now := nowText()
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "operator"
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE alerts
SET status = 'acknowledged', acknowledged_at = ?, acknowledged_by = ?, updated_at = ?
WHERE id = ? AND device_id = ? AND status = 'active'
`, now, actor, now, alertID, deviceID)
	if err != nil {
		return model.Alert{}, false, err
	}
	n, _ := res.RowsAffected()
	alert, found, err := s.GetAlert(ctx, deviceID, alertID)
	if err != nil || !found {
		return model.Alert{}, false, err
	}
	return alert, n > 0, nil
}

func (s *Store) GetAlert(ctx context.Context, deviceID, alertID string) (model.Alert, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, device_id, type, severity, status, message, details_json, first_seen_at, last_seen_at, resolved_at, acknowledged_at, acknowledged_by, created_at
FROM alerts
WHERE device_id = ? AND id = ?
`, deviceID, alertID)
	alert, err := scanAlert(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Alert{}, false, nil
	}
	if err != nil {
		return model.Alert{}, false, err
	}
	return alert, true, nil
}

func (s *Store) PurgeAlerts(ctx context.Context, opts PurgeOptions) (int64, error) {
	deviceID := strings.TrimSpace(opts.DeviceID)
	query := `DELETE FROM alerts`
	args := []any{}
	if deviceID != "" {
		query += ` WHERE device_id = ?`
		args = append(args, deviceID)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) CreateCommand(ctx context.Context, deviceID, commandType string, args json.RawMessage) (model.Command, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return model.Command{}, false, err
	}
	if !exists {
		return model.Command{}, false, nil
	}

	id, err := randomID("cmd")
	if err != nil {
		return model.Command{}, false, err
	}
	args = NormalizeRawJSON(args)
	now := nowText()
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano)

	_, err = s.db.ExecContext(ctx, `
INSERT INTO commands (id, device_id, type, args_json, status, max_attempts, created_at, expires_at)
VALUES (?, ?, ?, ?, 'queued', 3, ?, ?)
`, id, deviceID, commandType, string(args), now, expiresAt)
	if err != nil {
		return model.Command{}, false, err
	}

	return model.Command{
		ID:          id,
		DeviceID:    deviceID,
		Type:        commandType,
		Args:        args,
		Status:      "queued",
		Result:      json.RawMessage(`{}`),
		MaxAttempts: 3,
		CreatedAt:   parseTime(now),
		ExpiresAt:   ptrTime(parseTime(expiresAt)),
	}, true, nil
}

func (s *Store) ListCommands(ctx context.Context, deviceID string, opts CommandListOptions) ([]model.Command, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}

	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, device_id, type, args_json, status, result_json, output, exit_code, attempt_count, max_attempts, created_at, expires_at, claimed_at, completed_at, cancelled_at, expired_at
FROM commands
WHERE device_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?
`, deviceID, limit, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	commands := make([]model.Command, 0)
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, false, err
		}
		commands = append(commands, c)
	}
	return commands, true, rows.Err()
}

func (s *Store) GetCommand(ctx context.Context, deviceID, commandID string) (model.Command, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, device_id, type, args_json, status, result_json, output, exit_code, attempt_count, max_attempts, created_at, expires_at, claimed_at, completed_at, cancelled_at, expired_at
FROM commands
WHERE device_id = ? AND id = ?
`, deviceID, commandID)

	c, err := scanCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Command{}, false, nil
	}
	if err != nil {
		return model.Command{}, false, err
	}
	return c, true, nil
}

func (s *Store) CancelCommand(ctx context.Context, deviceID, commandID string) (model.Command, bool, error) {
	now := nowText()
	res, err := s.db.ExecContext(ctx, `
UPDATE commands
SET status = 'cancelled', cancelled_at = ?
WHERE device_id = ? AND id = ? AND status IN ('queued', 'claimed')
`, now, deviceID, commandID)
	if err != nil {
		return model.Command{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c, found, err := s.GetCommand(ctx, deviceID, commandID)
		if err != nil || !found {
			return model.Command{}, false, err
		}
		return c, true, nil
	}
	return s.GetCommand(ctx, deviceID, commandID)
}

func (s *Store) PurgeCommands(ctx context.Context, opts PurgeOptions) (int64, error) {
	deviceID := strings.TrimSpace(opts.DeviceID)
	query := `DELETE FROM commands WHERE status NOT IN ('queued', 'claimed')`
	args := []any{}
	if deviceID != "" {
		query += ` AND device_id = ?`
		args = append(args, deviceID)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) CreateRemoteSession(ctx context.Context, session model.RemoteSession) (model.RemoteSession, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, session.DeviceID).Scan(&exists); err != nil {
		return model.RemoteSession{}, false, err
	}
	if !exists {
		return model.RemoteSession{}, false, nil
	}

	id, err := randomID("ras")
	if err != nil {
		return model.RemoteSession{}, false, err
	}
	now := nowText()
	session.ID = id
	session.Status = strings.TrimSpace(session.Status)
	if session.Status == "" {
		session.Status = "requested"
	}
	session.Target = strings.TrimSpace(session.Target)
	if session.Target == "" {
		session.Target = "ssh"
	}
	session.LocalHost = strings.TrimSpace(session.LocalHost)
	if session.LocalHost == "" {
		session.LocalHost = "127.0.0.1"
	}
	if session.LocalPort <= 0 {
		session.LocalPort = 22
	}
	if session.ServerPort <= 0 {
		session.ServerPort = 22
	}
	if session.LuCIScheme == "" {
		session.LuCIScheme = "http"
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = parseTime(now)
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = time.Now().UTC().Add(15 * time.Minute)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO remote_sessions (id, device_id, target, status, server_host, server_port, remote_port, luci_port, luci_scheme, local_host, local_port, command_id, created_at, expires_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, session.ID, session.DeviceID, session.Target, session.Status, session.ServerHost, session.ServerPort, session.RemotePort, session.LuCIPort, session.LuCIScheme, session.LocalHost, session.LocalPort, session.CommandID, session.CreatedAt.Format(time.RFC3339Nano), session.ExpiresAt.Format(time.RFC3339Nano), now)
	if err != nil {
		return model.RemoteSession{}, false, err
	}
	return session, true, nil
}

func (s *Store) ListRemoteSessions(ctx context.Context, deviceID string, opts RemoteSessionListOptions) ([]model.RemoteSession, bool, error) {
	if err := s.ExpireRemoteSessions(ctx); err != nil {
		return nil, false, err
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, device_id, target, status, server_host, server_port, remote_port, luci_port, luci_scheme, local_host, local_port, command_id, created_at, expires_at, started_at, closed_at
FROM remote_sessions
WHERE device_id = ?
ORDER BY created_at DESC
LIMIT ?
`, deviceID, limit)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	sessions := make([]model.RemoteSession, 0)
	for rows.Next() {
		session, err := scanRemoteSession(rows)
		if err != nil {
			return nil, false, err
		}
		sessions = append(sessions, session)
	}
	return sessions, true, rows.Err()
}

func (s *Store) GetRemoteSession(ctx context.Context, deviceID, sessionID string) (model.RemoteSession, bool, error) {
	if err := s.ExpireRemoteSessions(ctx); err != nil {
		return model.RemoteSession{}, false, err
	}
	row := s.db.QueryRowContext(ctx, `
SELECT id, device_id, target, status, server_host, server_port, remote_port, luci_port, luci_scheme, local_host, local_port, command_id, created_at, expires_at, started_at, closed_at
FROM remote_sessions
WHERE device_id = ? AND id = ?
`, deviceID, sessionID)
	session, err := scanRemoteSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RemoteSession{}, false, nil
	}
	if err != nil {
		return model.RemoteSession{}, false, err
	}
	return session, true, nil
}

func (s *Store) AttachRemoteSessionCommand(ctx context.Context, deviceID, sessionID, commandID string) (model.RemoteSession, bool, error) {
	now := nowText()
	res, err := s.db.ExecContext(ctx, `
UPDATE remote_sessions
SET status = 'queued', command_id = ?, updated_at = ?
WHERE device_id = ? AND id = ? AND status IN ('requested', 'queued')
`, commandID, now, deviceID, sessionID)
	if err != nil {
		return model.RemoteSession{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return s.GetRemoteSession(ctx, deviceID, sessionID)
	}
	return s.GetRemoteSession(ctx, deviceID, sessionID)
}

func (s *Store) CloseRemoteSession(ctx context.Context, deviceID, sessionID string) (model.RemoteSession, bool, error) {
	now := nowText()
	res, err := s.db.ExecContext(ctx, `
UPDATE remote_sessions
SET status = 'closed', closed_at = ?, updated_at = ?
WHERE device_id = ? AND id = ? AND status IN ('requested', 'queued', 'active')
`, now, now, deviceID, sessionID)
	if err != nil {
		return model.RemoteSession{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return s.GetRemoteSession(ctx, deviceID, sessionID)
	}
	return s.GetRemoteSession(ctx, deviceID, sessionID)
}

func (s *Store) ExpireRemoteSessions(ctx context.Context) error {
	now := nowText()
	_, err := s.db.ExecContext(ctx, `
UPDATE remote_sessions
SET status = 'expired', closed_at = ?, updated_at = ?
WHERE status IN ('requested', 'queued', 'active') AND julianday(expires_at) <= julianday(?)
`, now, now, now)
	return err
}

func (s *Store) AddAuditEvent(ctx context.Context, actor, action, deviceID, commandID string, details json.RawMessage) (model.AuditEvent, error) {
	id, err := randomID("aud")
	if err != nil {
		return model.AuditEvent{}, err
	}
	details = NormalizeRawJSON(details)
	now := nowText()

	_, err = s.db.ExecContext(ctx, `
INSERT INTO audit_events (id, actor, action, device_id, command_id, details_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, id, actor, action, deviceID, commandID, string(details), now)
	if err != nil {
		return model.AuditEvent{}, err
	}

	return model.AuditEvent{
		ID:        id,
		Actor:     actor,
		Action:    action,
		DeviceID:  deviceID,
		CommandID: commandID,
		Details:   details,
		CreatedAt: parseTime(now),
	}, nil
}

func (s *Store) ListAuditEvents(ctx context.Context, opts AuditListOptions) ([]model.AuditEvent, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := `
SELECT id, actor, action, device_id, command_id, details_json, created_at
FROM audit_events
`
	args := []any{}
	if strings.TrimSpace(opts.DeviceID) != "" {
		query += `WHERE device_id = ?` + "\n"
		args = append(args, opts.DeviceID)
	}
	query += `ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]model.AuditEvent, 0)
	for rows.Next() {
		var e model.AuditEvent
		var details string
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.DeviceID, &e.CommandID, &details, &createdAt); err != nil {
			return nil, err
		}
		e.Details = json.RawMessage(details)
		e.CreatedAt = parseTime(createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) PurgeAuditEvents(ctx context.Context, opts PurgeOptions) (int64, error) {
	deviceID := strings.TrimSpace(opts.DeviceID)
	query := `DELETE FROM audit_events`
	args := []any{}
	if deviceID != "" {
		query += ` WHERE device_id = ?`
		args = append(args, deviceID)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDevice(s scanner) (model.Device, error) {
	var d model.Device
	var lastSeen sql.NullString
	var createdAt string
	var inventory string
	var metrics string
	var tags string
	if err := s.Scan(&d.ID, &d.OwnerUserID, &d.DNSLabel, &d.Hostname, &d.OpenWrtVersion, &lastSeen, &createdAt, &inventory, &metrics, &d.Group, &tags, &d.ActiveAlerts); err != nil {
		return d, err
	}
	d.CreatedAt = parseTime(createdAt)
	d.Inventory = json.RawMessage(inventory)
	d.Metrics = json.RawMessage(metrics)
	if err := json.Unmarshal([]byte(tags), &d.Tags); err != nil {
		d.Tags = []string{}
	}
	if lastSeen.Valid {
		t := parseTime(lastSeen.String)
		d.LastSeenAt = &t
		d.Online = time.Since(t) < 2*time.Minute
	}
	return d, nil
}

func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized
}

func scanCommand(s scanner) (model.Command, error) {
	var c model.Command
	var createdAt string
	var expiresAt sql.NullString
	var claimedAt sql.NullString
	var completedAt sql.NullString
	var cancelledAt sql.NullString
	var expiredAt sql.NullString
	var args string
	var result string
	var exitCode sql.NullInt64
	if err := s.Scan(
		&c.ID,
		&c.DeviceID,
		&c.Type,
		&args,
		&c.Status,
		&result,
		&c.Output,
		&exitCode,
		&c.AttemptCount,
		&c.MaxAttempts,
		&createdAt,
		&expiresAt,
		&claimedAt,
		&completedAt,
		&cancelledAt,
		&expiredAt,
	); err != nil {
		return c, err
	}
	c.Args = json.RawMessage(args)
	c.Result = json.RawMessage(result)
	if exitCode.Valid {
		v := int(exitCode.Int64)
		c.ExitCode = &v
	}
	c.CreatedAt = parseTime(createdAt)
	if expiresAt.Valid {
		t := parseTime(expiresAt.String)
		c.ExpiresAt = &t
	}
	if claimedAt.Valid {
		t := parseTime(claimedAt.String)
		c.ClaimedAt = &t
	}
	if completedAt.Valid {
		t := parseTime(completedAt.String)
		c.CompletedAt = &t
	}
	if cancelledAt.Valid {
		t := parseTime(cancelledAt.String)
		c.CancelledAt = &t
	}
	if expiredAt.Valid {
		t := parseTime(expiredAt.String)
		c.ExpiredAt = &t
	}
	return c, nil
}

func scanAlert(s scanner) (model.Alert, error) {
	var alert model.Alert
	var details string
	var firstSeenAt string
	var lastSeenAt string
	var resolvedAt sql.NullString
	var acknowledgedAt sql.NullString
	var createdAt string
	if err := s.Scan(
		&alert.ID,
		&alert.DeviceID,
		&alert.Type,
		&alert.Severity,
		&alert.Status,
		&alert.Message,
		&details,
		&firstSeenAt,
		&lastSeenAt,
		&resolvedAt,
		&acknowledgedAt,
		&alert.AcknowledgedBy,
		&createdAt,
	); err != nil {
		return alert, err
	}
	alert.Details = json.RawMessage(details)
	alert.FirstSeenAt = parseTime(firstSeenAt)
	alert.LastSeenAt = parseTime(lastSeenAt)
	alert.CreatedAt = parseTime(createdAt)
	if resolvedAt.Valid {
		t := parseTime(resolvedAt.String)
		alert.ResolvedAt = &t
	}
	if acknowledgedAt.Valid {
		t := parseTime(acknowledgedAt.String)
		alert.AcknowledgedAt = &t
	}
	return alert, nil
}

func scanRemoteSession(s scanner) (model.RemoteSession, error) {
	var session model.RemoteSession
	var createdAt string
	var expiresAt string
	var startedAt sql.NullString
	var closedAt sql.NullString
	if err := s.Scan(
		&session.ID,
		&session.DeviceID,
		&session.Target,
		&session.Status,
		&session.ServerHost,
		&session.ServerPort,
		&session.RemotePort,
		&session.LuCIPort,
		&session.LuCIScheme,
		&session.LocalHost,
		&session.LocalPort,
		&session.CommandID,
		&createdAt,
		&expiresAt,
		&startedAt,
		&closedAt,
	); err != nil {
		return session, err
	}
	session.CreatedAt = parseTime(createdAt)
	session.ExpiresAt = parseTime(expiresAt)
	if startedAt.Valid {
		t := parseTime(startedAt.String)
		session.StartedAt = &t
	}
	if closedAt.Valid {
		t := parseTime(closedAt.String)
		session.ClosedAt = &t
	}
	return session, nil
}

func NormalizeRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return json.RawMessage(`{}`)
	}
	return compact.Bytes()
}

func nowText() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) migrateDeviceTokens(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, token FROM devices WHERE token_hash = ''`)
	if err != nil {
		return err
	}
	type legacyToken struct{ id, token string }
	legacy := make([]legacyToken, 0)
	for rows.Next() {
		var item legacyToken
		if err := rows.Scan(&item.id, &item.token); err != nil {
			rows.Close()
			return err
		}
		legacy = append(legacy, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range legacy {
		if _, err := s.db.ExecContext(ctx, `UPDATE devices SET token = ?, token_hash = ? WHERE id = ?`, "redacted:"+item.id, TokenHash(item.token), item.id); err != nil {
			return err
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}

func RedactSensitiveOutput(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lines[i] = redactSensitiveLine(line)
	}
	return strings.Join(lines, "\n")
}

func redactSensitiveLine(line string) string {
	keys := []string{"private_key", "password", "passwd", "secret", "psk", "token", "key"}
	trimmed := strings.TrimSpace(line)
	fields := strings.Fields(strings.ToLower(trimmed))
	if len(fields) >= 3 && fields[0] == "option" {
		for _, key := range keys {
			if fields[1] == key {
				index := strings.Index(strings.ToLower(line), key)
				return line[:index+len(key)] + " '[redacted]'"
			}
		}
	}
	if eq := strings.Index(line, "="); eq >= 0 {
		left := strings.ToLower(strings.TrimSpace(line[:eq]))
		for _, key := range keys {
			if left == key || strings.HasSuffix(left, "."+key) || strings.HasSuffix(left, "_"+key) || strings.HasSuffix(left, "-"+key) {
				return line[:eq+1] + "'[redacted]'"
			}
		}
	}
	return line
}
