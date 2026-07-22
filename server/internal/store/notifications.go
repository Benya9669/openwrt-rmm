package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"rmm-openwrt/server/internal/model"
)

func DefaultNotificationSettings(userID string) model.NotificationSettings {
	return model.NotificationSettings{
		UserID:                 userID,
		NotifyWarning:          true,
		NotifyCritical:         true,
		NotifyResolved:         true,
		MemoryThresholdPercent: 85,
		DiskThresholdPercent:   85,
		PacketLossPercent:      20,
		LatencyThresholdMS:     200,
	}
}

func (s *Store) GetNotificationSettings(ctx context.Context, userID string) (model.NotificationSettings, bool, error) {
	settings := DefaultNotificationSettings(userID)
	var emailEnabled, telegramEnabled, notifyWarning, notifyCritical, notifyResolved int
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
SELECT email_enabled, telegram_enabled, telegram_chat_id, notify_warning, notify_critical, notify_resolved,
       memory_threshold_percent, disk_threshold_percent, packet_loss_percent, latency_threshold_ms,
       repeat_minutes, created_at, updated_at
FROM notification_settings WHERE user_id = ?
`, userID).Scan(
		&emailEnabled, &telegramEnabled, &settings.TelegramChatID, &notifyWarning, &notifyCritical, &notifyResolved,
		&settings.MemoryThresholdPercent, &settings.DiskThresholdPercent, &settings.PacketLossPercent,
		&settings.LatencyThresholdMS, &settings.RepeatMinutes, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, false, nil
	}
	if err != nil {
		return model.NotificationSettings{}, false, err
	}
	settings.Configured = true
	settings.EmailEnabled = emailEnabled != 0
	settings.TelegramEnabled = telegramEnabled != 0
	settings.NotifyWarning = notifyWarning != 0
	settings.NotifyCritical = notifyCritical != 0
	settings.NotifyResolved = notifyResolved != 0
	settings.CreatedAt = parseTime(createdAt)
	settings.UpdatedAt = parseTime(updatedAt)
	return settings, true, nil
}

func (s *Store) UpsertNotificationSettings(ctx context.Context, settings model.NotificationSettings) (model.NotificationSettings, error) {
	now := nowText()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO notification_settings (
  user_id, email_enabled, telegram_enabled, telegram_chat_id, notify_warning, notify_critical,
  notify_resolved, memory_threshold_percent, disk_threshold_percent, packet_loss_percent,
  latency_threshold_ms, repeat_minutes, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
  email_enabled = excluded.email_enabled,
  telegram_enabled = excluded.telegram_enabled,
  telegram_chat_id = excluded.telegram_chat_id,
  notify_warning = excluded.notify_warning,
  notify_critical = excluded.notify_critical,
  notify_resolved = excluded.notify_resolved,
  memory_threshold_percent = excluded.memory_threshold_percent,
  disk_threshold_percent = excluded.disk_threshold_percent,
  packet_loss_percent = excluded.packet_loss_percent,
  latency_threshold_ms = excluded.latency_threshold_ms,
  repeat_minutes = excluded.repeat_minutes,
  updated_at = excluded.updated_at
`, settings.UserID, boolInt(settings.EmailEnabled), boolInt(settings.TelegramEnabled), strings.TrimSpace(settings.TelegramChatID),
		boolInt(settings.NotifyWarning), boolInt(settings.NotifyCritical), boolInt(settings.NotifyResolved),
		settings.MemoryThresholdPercent, settings.DiskThresholdPercent, settings.PacketLossPercent,
		settings.LatencyThresholdMS, settings.RepeatMinutes, now, now)
	if err != nil {
		return model.NotificationSettings{}, err
	}
	stored, _, err := s.GetNotificationSettings(ctx, settings.UserID)
	return stored, err
}

func (s *Store) CreateNotificationDelivery(ctx context.Context, delivery model.NotificationDelivery, dedupeKey string) (model.NotificationDelivery, bool, error) {
	id, err := randomID("ntf")
	if err != nil {
		return model.NotificationDelivery{}, false, err
	}
	now := nowText()
	var deviceID any
	if strings.TrimSpace(delivery.DeviceID) != "" {
		deviceID = delivery.DeviceID
	}
	res, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO notification_deliveries (
  id, user_id, device_id, alert_id, dedupe_key, event, channel, status, title, body,
  destination, destination_masked, error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, 'queued', ?, ?, ?, ?, '', ?, ?)
`, id, delivery.UserID, deviceID, delivery.AlertID, dedupeKey, delivery.Event, delivery.Channel,
		delivery.Title, delivery.Body, delivery.Destination, delivery.DestinationMasked, now, now)
	if err != nil {
		return model.NotificationDelivery{}, false, err
	}
	inserted, _ := res.RowsAffected()
	if inserted == 0 {
		return model.NotificationDelivery{}, false, nil
	}
	delivery.ID = id
	delivery.Status = "queued"
	delivery.CreatedAt = parseTime(now)
	return delivery, true, nil
}

func (s *Store) CompleteNotificationDelivery(ctx context.Context, id, status, errorMessage string) error {
	status = strings.TrimSpace(status)
	if status != "sent" && status != "failed" {
		return errors.New("invalid notification delivery status")
	}
	now := nowText()
	var sentAt any
	if status == "sent" {
		sentAt = now
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE notification_deliveries
SET status = ?, error = ?, sent_at = ?, updated_at = ?
WHERE id = ? AND status = 'queued'
`, status, strings.TrimSpace(errorMessage), sentAt, now, id)
	return err
}

func (s *Store) LatestNotificationDelivery(ctx context.Context, userID, alertID, channel string) (model.NotificationDelivery, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, user_id, COALESCE(device_id, ''), alert_id, event, channel, status, title, body,
       destination, destination_masked, error, created_at, sent_at
FROM notification_deliveries
WHERE user_id = ? AND alert_id = ? AND channel = ? AND event IN ('active', 'repeat')
ORDER BY created_at DESC LIMIT 1
`, userID, alertID, channel)
	delivery, err := scanNotificationDelivery(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.NotificationDelivery{}, false, nil
	}
	return delivery, err == nil, err
}

func (s *Store) ListNotificationDeliveries(ctx context.Context, opts NotificationListOptions) ([]model.NotificationDelivery, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, COALESCE(device_id, ''), alert_id, event, channel, status, title, body,
       destination, destination_masked, error, created_at, sent_at
FROM notification_deliveries
WHERE user_id = ?
ORDER BY created_at DESC LIMIT ?
`, opts.UserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	deliveries := make([]model.NotificationDelivery, 0)
	for rows.Next() {
		delivery, err := scanNotificationDelivery(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanNotificationDelivery(scanner notificationScanner) (model.NotificationDelivery, error) {
	var delivery model.NotificationDelivery
	var createdAt string
	var sentAt sql.NullString
	err := scanner.Scan(
		&delivery.ID, &delivery.UserID, &delivery.DeviceID, &delivery.AlertID, &delivery.Event,
		&delivery.Channel, &delivery.Status, &delivery.Title, &delivery.Body, &delivery.Destination,
		&delivery.DestinationMasked, &delivery.Error, &createdAt, &sentAt,
	)
	if err != nil {
		return model.NotificationDelivery{}, err
	}
	delivery.CreatedAt = parseTime(createdAt)
	if sentAt.Valid {
		value := parseTime(sentAt.String)
		delivery.SentAt = &value
	}
	return delivery, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
