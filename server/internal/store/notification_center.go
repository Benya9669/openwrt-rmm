package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"rmm-openwrt/server/internal/model"
)

func (s *Store) CreateInboxNotification(ctx context.Context, notification model.InboxNotification, dedupeKey string) (model.InboxNotification, bool, error) {
	id, err := randomID("inb")
	if err != nil {
		return model.InboxNotification{}, false, err
	}
	now := nowText()
	res, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO inbox_notifications
  (id, user_id, device_id, incident_id, dedupe_key, severity, event, title, body, created_at)
VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)
`, id, notification.UserID, notification.DeviceID, notification.IncidentID, dedupeKey,
		notification.Severity, notification.Event, notification.Title, notification.Body, now)
	if err != nil {
		return model.InboxNotification{}, false, err
	}
	inserted, _ := res.RowsAffected()
	if inserted == 0 {
		return model.InboxNotification{}, false, nil
	}
	notification.ID = id
	notification.CreatedAt = parseTime(now)
	return notification, true, nil
}

func (s *Store) ListInboxNotifications(ctx context.Context, userID string, limit int) ([]model.InboxNotification, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var unread int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM inbox_notifications WHERE user_id = ? AND read_at IS NULL`, userID).Scan(&unread); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, COALESCE(device_id, ''), incident_id, severity, event, title, body, read_at, created_at
FROM inbox_notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT ?
`, userID, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]model.InboxNotification, 0)
	for rows.Next() {
		var item model.InboxNotification
		var readAt sql.NullString
		var createdAt string
		if err := rows.Scan(&item.ID, &item.UserID, &item.DeviceID, &item.IncidentID, &item.Severity,
			&item.Event, &item.Title, &item.Body, &readAt, &createdAt); err != nil {
			return nil, 0, err
		}
		item.CreatedAt = parseTime(createdAt)
		if readAt.Valid {
			value := parseTime(readAt.String)
			item.ReadAt = &value
		}
		items = append(items, item)
	}
	return items, unread, rows.Err()
}

func (s *Store) MarkInboxNotificationRead(ctx context.Context, userID, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE inbox_notifications SET read_at = COALESCE(read_at, ?) WHERE id = ? AND user_id = ?
`, nowText(), id, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) MarkAllInboxNotificationsRead(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE inbox_notifications SET read_at = COALESCE(read_at, ?) WHERE user_id = ?`, nowText(), userID)
	return err
}

func (s *Store) GetDeviceNotificationSettings(ctx context.Context, userID, deviceID string) (model.DeviceNotificationSettings, bool, error) {
	settings := model.DeviceNotificationSettings{
		DeviceID: deviceID, Enabled: true, NotifyWarning: true, NotifyCritical: true, NotifyResolved: true,
	}
	var enabled, warning, critical, resolved int
	var paused sql.NullString
	var updated string
	err := s.db.QueryRowContext(ctx, `
SELECT enabled, notify_warning, notify_critical, notify_resolved, paused_until, updated_at
FROM device_notification_settings WHERE user_id = ? AND device_id = ?
`, userID, deviceID).Scan(&enabled, &warning, &critical, &resolved, &paused, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, false, nil
	}
	if err != nil {
		return model.DeviceNotificationSettings{}, false, err
	}
	settings.Enabled = enabled != 0
	settings.NotifyWarning = warning != 0
	settings.NotifyCritical = critical != 0
	settings.NotifyResolved = resolved != 0
	settings.UpdatedAt = parseTime(updated)
	if paused.Valid && paused.String != "" {
		value := parseTime(paused.String)
		settings.PausedUntil = &value
	}
	return settings, true, nil
}

func (s *Store) UpsertDeviceNotificationSettings(ctx context.Context, userID string, settings model.DeviceNotificationSettings) (model.DeviceNotificationSettings, error) {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO device_notification_settings
  (user_id, device_id, enabled, notify_warning, notify_critical, notify_resolved, paused_until, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, device_id) DO UPDATE SET
  enabled = excluded.enabled, notify_warning = excluded.notify_warning,
  notify_critical = excluded.notify_critical, notify_resolved = excluded.notify_resolved,
  paused_until = excluded.paused_until, updated_at = excluded.updated_at
`, userID, settings.DeviceID, boolInt(settings.Enabled), boolInt(settings.NotifyWarning),
		boolInt(settings.NotifyCritical), boolInt(settings.NotifyResolved), nullableTime(settings.PausedUntil), nowText())
	if err != nil {
		return model.DeviceNotificationSettings{}, err
	}
	stored, _, err := s.GetDeviceNotificationSettings(ctx, userID, settings.DeviceID)
	return stored, err
}

func inboxRetentionCutoff(days int) time.Time {
	return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
}
