package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
)

type AccessRoute struct {
	UserID          string
	DeviceID        string
	DNSLabel        string
	RemoteSessionID string
	ExpiresAt       time.Time
}

func (s *Store) EnsureBootstrapUser(ctx context.Context, username, passwordHash string) (model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || passwordHash == "" {
		return model.User{}, errors.New("bootstrap username and password hash are required")
	}
	if user, _, found, err := s.GetUserByUsername(ctx, username); err != nil {
		return model.User{}, err
	} else if found {
		// The environment password is a first-run bootstrap secret. Preserve a
		// password changed from the account UI on subsequent server restarts.
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET role = 'admin', disabled = 0, updated_at = ? WHERE id = ?`, nowText(), user.ID); err != nil {
			return model.User{}, err
		}
		if err := s.assignLegacyDevices(ctx, user.ID); err != nil {
			return model.User{}, err
		}
		user.Role = "admin"
		user.Disabled = false
		return user, nil
	}

	id, err := randomID("usr")
	if err != nil {
		return model.User{}, err
	}
	now := nowText()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO users (id, username, password_hash, role, disabled, created_at, updated_at)
VALUES (?, ?, ?, 'admin', 0, ?, ?)
`, id, username, passwordHash, now, now)
	if err != nil {
		return model.User{}, err
	}
	if err := s.assignLegacyDevices(ctx, id); err != nil {
		return model.User{}, err
	}
	user, _, found, err := s.GetUserByUsername(ctx, username)
	if err != nil || !found {
		return model.User{}, err
	}
	return user, nil
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, role string) (model.User, error) {
	username = strings.TrimSpace(username)
	role = strings.ToLower(strings.TrimSpace(role))
	if username == "" || passwordHash == "" || (role != "admin" && role != "user") {
		return model.User{}, errors.New("invalid user")
	}
	id, err := randomID("usr")
	if err != nil {
		return model.User{}, err
	}
	now := nowText()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO users (id, username, password_hash, role, disabled, created_at, updated_at)
VALUES (?, ?, ?, ?, 0, ?, ?)
`, id, username, passwordHash, role, now, now)
	if err != nil {
		return model.User{}, err
	}
	user, _, found, err := s.GetUserByUsername(ctx, username)
	if err != nil || !found {
		return model.User{}, err
	}
	return user, nil
}

func (s *Store) UpdateUserSecurity(ctx context.Context, userID string, disabled *bool, passwordHash string) (model.User, bool, error) {
	user, found, err := s.GetUserByID(ctx, userID)
	if err != nil || !found {
		return model.User{}, found, err
	}
	if disabled != nil && *disabled && user.Role == "admin" {
		var activeAdmins int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE role = 'admin' AND disabled = 0`).Scan(&activeAdmins); err != nil {
			return model.User{}, false, err
		}
		if activeAdmins <= 1 && !user.Disabled {
			return model.User{}, false, errors.New("cannot disable the last active administrator")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, false, err
	}
	defer tx.Rollback()
	if disabled != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET disabled = ?, updated_at = ? WHERE id = ?`, *disabled, nowText(), userID); err != nil {
			return model.User{}, false, err
		}
	}
	if passwordHash != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, nowText(), userID); err != nil {
			return model.User{}, false, err
		}
	}
	if disabled != nil && *disabled || passwordHash != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM operator_sessions WHERE user_id = ?`, userID); err != nil {
			return model.User{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.User{}, false, err
	}
	updated, found, err := s.GetUserByID(ctx, userID)
	return updated, found, err
}

func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, username, display_name, email, role, disabled, created_at, updated_at
FROM users
ORDER BY username COLLATE NOCASE
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]model.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (model.User, string, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, display_name, email, role, disabled, created_at, updated_at, password_hash
FROM users WHERE username = ? COLLATE NOCASE
`, strings.TrimSpace(username))
	var user model.User
	var disabled int
	var createdAt, updatedAt, passwordHash string
	if err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.Role, &disabled, &createdAt, &updatedAt, &passwordHash); errors.Is(err, sql.ErrNoRows) {
		return model.User{}, "", false, nil
	} else if err != nil {
		return model.User{}, "", false, err
	}
	user.Disabled = disabled != 0
	user.CreatedAt = parseTime(createdAt)
	user.UpdatedAt = parseTime(updatedAt)
	return user, passwordHash, true, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (model.User, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, display_name, email, role, disabled, created_at, updated_at FROM users WHERE id = ?
`, id)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	return user, err == nil, err
}

func (s *Store) CreateOperatorSession(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO operator_sessions (token_hash, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?)
`, tokenHash, userID, expiresAt.UTC().Format(time.RFC3339Nano), nowText())
	return err
}

func (s *Store) AuthorizeOperatorSession(ctx context.Context, tokenHash string) (model.User, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT u.id, u.username, u.display_name, u.email, u.role, u.disabled, u.created_at, u.updated_at
FROM operator_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ? AND julianday(s.expires_at) > julianday(?) AND u.disabled = 0
`, tokenHash, nowText())
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	return user, err == nil, err
}

func (s *Store) RevokeOperatorSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM operator_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) UpdateUserProfile(ctx context.Context, userID, displayName, email string) (model.User, bool, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE users SET display_name = ?, email = ?, updated_at = ? WHERE id = ?
`, strings.TrimSpace(displayName), strings.ToLower(strings.TrimSpace(email)), nowText(), userID)
	if err != nil {
		return model.User{}, false, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed == 0 {
		return model.User{}, false, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) UpdateOwnPassword(ctx context.Context, userID, passwordHash, currentSessionHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, nowText(), userID); err != nil {
		return err
	}
	if currentSessionHash == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM operator_sessions WHERE user_id = ?`, userID)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM operator_sessions WHERE user_id = ? AND token_hash != ?`, userID, currentSessionHash)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RevokeUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM operator_sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) CreateEnrollmentGrant(ctx context.Context, userID, dnsLabel, tokenHash string, expiresAt time.Time) (model.EnrollmentGrant, error) {
	var activeGrants int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM enrollment_grants WHERE user_id = ? AND used_at IS NULL AND julianday(expires_at) > julianday(?)`, userID, nowText()).Scan(&activeGrants); err != nil {
		return model.EnrollmentGrant{}, err
	}
	if activeGrants >= 10 {
		return model.EnrollmentGrant{}, errors.New("active enrollment grant limit reached")
	}
	if dnsLabel != "" {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1 FROM devices WHERE dns_label = ?
	UNION ALL
	SELECT 1 FROM enrollment_grants WHERE dns_label = ? AND used_at IS NULL AND julianday(expires_at) > julianday(?)
)
`, dnsLabel, dnsLabel, nowText()).Scan(&exists); err != nil {
			return model.EnrollmentGrant{}, err
		}
		if exists {
			return model.EnrollmentGrant{}, fmt.Errorf("dns label is already in use")
		}
	}
	id, err := randomID("eng")
	if err != nil {
		return model.EnrollmentGrant{}, err
	}
	now := time.Now().UTC()
	grant := model.EnrollmentGrant{ID: id, UserID: userID, DNSLabel: dnsLabel, ExpiresAt: expiresAt.UTC(), CreatedAt: now}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO enrollment_grants (id, token_hash, user_id, dns_label, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)
`, grant.ID, tokenHash, userID, dnsLabel, grant.ExpiresAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return grant, err
}

func (s *Store) EnrollDeviceWithGrant(ctx context.Context, tokenHash, hostname, openwrtVersion string) (EnrolledDevice, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EnrolledDevice{}, false, err
	}
	defer tx.Rollback()
	var grantID, userID, dnsLabel string
	err = tx.QueryRowContext(ctx, `
SELECT g.id, g.user_id, g.dns_label
FROM enrollment_grants g
JOIN users u ON u.id = g.user_id
WHERE g.token_hash = ? AND g.used_at IS NULL AND julianday(g.expires_at) > julianday(?) AND u.disabled = 0
`, tokenHash, nowText()).Scan(&grantID, &userID, &dnsLabel)
	if errors.Is(err, sql.ErrNoRows) {
		return EnrolledDevice{}, false, nil
	}
	if err != nil {
		return EnrolledDevice{}, false, err
	}

	id, err := randomID("dev")
	if err != nil {
		return EnrolledDevice{}, false, err
	}
	deviceToken, err := randomID("tok")
	if err != nil {
		return EnrolledDevice{}, false, err
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown"
	}
	openwrtVersion = strings.TrimSpace(openwrtVersion)
	if openwrtVersion == "" {
		openwrtVersion = "unknown"
	}
	if dnsLabel == "" {
		dnsLabel = generatedDNSLabel(hostname, id)
	}
	now := nowText()
	result, err := tx.ExecContext(ctx, `
UPDATE enrollment_grants SET used_at = ? WHERE id = ? AND used_at IS NULL
`, now, grantID)
	if err != nil {
		return EnrolledDevice{}, false, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return EnrolledDevice{}, false, nil
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO devices (id, token, token_hash, owner_user_id, dns_label, hostname, openwrt_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, id, "redacted:"+id, TokenHash(deviceToken), userID, dnsLabel, hostname, openwrtVersion, now)
	if err != nil {
		return EnrolledDevice{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return EnrolledDevice{}, false, err
	}
	return EnrolledDevice{DeviceID: id, DeviceToken: deviceToken}, true, nil
}

func (s *Store) ListDevicesForUser(ctx context.Context, userID string, admin bool) ([]model.Device, error) {
	query := deviceSelect + ` ORDER BY d.created_at DESC`
	args := []any{}
	if !admin {
		query = deviceSelect + ` WHERE d.owner_user_id = ? ORDER BY d.created_at DESC`
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := make([]model.Device, 0)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (s *Store) DeviceAccessible(ctx context.Context, deviceID, userID string, admin bool) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`
	args := []any{deviceID}
	if !admin {
		query = `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ? AND owner_user_id = ?)`
		args = append(args, userID)
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&exists)
	return exists, err
}

func (s *Store) GetDeviceByDNSLabel(ctx context.Context, dnsLabel string) (model.Device, bool, error) {
	row := s.db.QueryRowContext(ctx, deviceSelect+` WHERE d.dns_label = ?`, dnsLabel)
	device, err := scanDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Device{}, false, nil
	}
	return device, err == nil, err
}

func (s *Store) UpdateDeviceDNSLabel(ctx context.Context, deviceID, dnsLabel string) (model.Device, bool, error) {
	var reserved bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM enrollment_grants WHERE dns_label = ? AND used_at IS NULL AND julianday(expires_at) > julianday(?))`, dnsLabel, nowText()).Scan(&reserved); err != nil {
		return model.Device{}, false, err
	}
	if reserved {
		return model.Device{}, false, errors.New("dns label is reserved by an active enrollment grant")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE devices SET dns_label = ? WHERE id = ?`, dnsLabel, deviceID)
	if err != nil {
		return model.Device{}, false, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return model.Device{}, false, nil
	}
	return s.GetDevice(ctx, deviceID)
}

func (s *Store) CreateDeviceAccessGrant(ctx context.Context, tokenHash, userID, deviceID, remoteSessionID string, expiresAt time.Time) error {
	var activeGrants int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM device_access_grants WHERE user_id = ? AND julianday(expires_at) > julianday(?)`, userID, nowText()).Scan(&activeGrants); err != nil {
		return err
	}
	if activeGrants >= 20 {
		return errors.New("active device access grant limit reached")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO device_access_grants (token_hash, user_id, device_id, remote_session_id, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)
`, tokenHash, userID, deviceID, remoteSessionID, expiresAt.UTC().Format(time.RFC3339Nano), nowText())
	return err
}

func (s *Store) ConsumeDeviceAccessGrant(ctx context.Context, grantHash, sessionHash, dnsLabel string, sessionExpiresAt time.Time) (AccessRoute, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AccessRoute{}, false, err
	}
	defer tx.Rollback()
	var route AccessRoute
	var grantExpires, remoteExpires string
	err = tx.QueryRowContext(ctx, `
SELECT g.user_id, g.device_id, d.dns_label, g.remote_session_id, g.expires_at, rs.expires_at
FROM device_access_grants g
JOIN devices d ON d.id = g.device_id
JOIN users u ON u.id = g.user_id
JOIN remote_sessions rs ON rs.id = g.remote_session_id AND rs.device_id = g.device_id
WHERE g.token_hash = ? AND d.dns_label = ? AND julianday(g.expires_at) > julianday(?) AND u.disabled = 0
  AND rs.status = 'active' AND julianday(rs.expires_at) > julianday(?)
`, grantHash, dnsLabel, nowText(), nowText()).Scan(&route.UserID, &route.DeviceID, &route.DNSLabel, &route.RemoteSessionID, &grantExpires, &remoteExpires)
	if errors.Is(err, sql.ErrNoRows) {
		return AccessRoute{}, false, nil
	}
	if err != nil {
		return AccessRoute{}, false, err
	}
	if remoteExpiry := parseTime(remoteExpires); sessionExpiresAt.After(remoteExpiry) {
		sessionExpiresAt = remoteExpiry
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM device_access_grants WHERE token_hash = ?`, grantHash)
	if err != nil {
		return AccessRoute{}, false, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return AccessRoute{}, false, nil
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO device_access_sessions (token_hash, user_id, device_id, remote_session_id, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)
`, sessionHash, route.UserID, route.DeviceID, route.RemoteSessionID, sessionExpiresAt.UTC().Format(time.RFC3339Nano), nowText())
	if err != nil {
		return AccessRoute{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return AccessRoute{}, false, err
	}
	route.ExpiresAt = sessionExpiresAt.UTC()
	return route, true, nil
}

func (s *Store) AuthorizeDeviceAccessSession(ctx context.Context, sessionHash, dnsLabel string) (AccessRoute, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT s.user_id, s.device_id, d.dns_label, s.remote_session_id, s.expires_at
FROM device_access_sessions s
JOIN devices d ON d.id = s.device_id
JOIN users u ON u.id = s.user_id
JOIN remote_sessions rs ON rs.id = s.remote_session_id AND rs.device_id = s.device_id
WHERE s.token_hash = ? AND d.dns_label = ? AND julianday(s.expires_at) > julianday(?) AND u.disabled = 0
  AND rs.status = 'active' AND julianday(rs.expires_at) > julianday(?)
`, sessionHash, dnsLabel, nowText(), nowText())
	var route AccessRoute
	var expiresAt string
	if err := row.Scan(&route.UserID, &route.DeviceID, &route.DNSLabel, &route.RemoteSessionID, &expiresAt); errors.Is(err, sql.ErrNoRows) {
		return AccessRoute{}, false, nil
	} else if err != nil {
		return AccessRoute{}, false, err
	}
	route.ExpiresAt = parseTime(expiresAt)
	return route, true, nil
}

func (s *Store) PurgeExpiredSecurityData(ctx context.Context) error {
	now := nowText()
	for _, statement := range []string{
		`DELETE FROM operator_sessions WHERE julianday(expires_at) <= julianday(?)`,
		`DELETE FROM enrollment_grants WHERE julianday(expires_at) <= julianday(?)`,
		`DELETE FROM device_access_grants WHERE julianday(expires_at) <= julianday(?)`,
		`DELETE FROM device_access_sessions WHERE julianday(expires_at) <= julianday(?)`,
	} {
		if _, err := s.db.ExecContext(ctx, statement, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) PurgeMetricSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM metric_samples WHERE julianday(created_at) < julianday(?)`, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

const deviceSelect = `
SELECT d.id, d.owner_user_id, d.dns_label, d.hostname, d.openwrt_version, d.last_seen_at, d.created_at, d.inventory_json, d.metrics_json, d.group_name, d.tags_json,
	(SELECT COUNT(1) FROM alerts a WHERE a.device_id = d.id AND a.status IN ('active', 'acknowledged'))
FROM devices d`

func scanUser(row scanner) (model.User, error) {
	var user model.User
	var disabled int
	var createdAt, updatedAt string
	err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.Role, &disabled, &createdAt, &updatedAt)
	user.Disabled = disabled != 0
	user.CreatedAt = parseTime(createdAt)
	user.UpdatedAt = parseTime(updatedAt)
	return user, err
}

func (s *Store) assignLegacyDevices(ctx context.Context, userID string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, hostname FROM devices WHERE owner_user_id = '' OR dns_label = ''`)
	if err != nil {
		return err
	}
	type legacyDevice struct{ id, hostname string }
	devices := make([]legacyDevice, 0)
	for rows.Next() {
		var device legacyDevice
		if err := rows.Scan(&device.id, &device.hostname); err != nil {
			rows.Close()
			return err
		}
		devices = append(devices, device)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, device := range devices {
		label := generatedDNSLabel(device.hostname, device.id)
		if _, err := s.db.ExecContext(ctx, `
UPDATE devices
SET owner_user_id = CASE WHEN owner_user_id = '' THEN ? ELSE owner_user_id END,
    dns_label = CASE WHEN dns_label = '' THEN ? ELSE dns_label END
WHERE id = ?
`, userID, label, device.id); err != nil {
			return err
		}
	}
	return nil
}

func generatedDNSLabel(hostname, deviceID string) string {
	base := strings.ToLower(strings.TrimSpace(hostname))
	var builder strings.Builder
	lastDash := false
	for _, r := range base {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if valid {
			builder.WriteRune(r)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
		if builder.Len() >= 48 {
			break
		}
	}
	stem := strings.Trim(builder.String(), "-")
	if stem == "" {
		stem = "router"
	}
	suffix := strings.TrimPrefix(deviceID, "dev_")
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return strings.Trim(stem+"-"+suffix, "-")
}
