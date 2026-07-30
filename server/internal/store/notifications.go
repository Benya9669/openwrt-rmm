package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

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
		Timezone:               "UTC",
		QuietHoursStart:        "22:00",
		QuietHoursEnd:          "08:00",
	}
}

func (s *Store) GetNotificationSettings(ctx context.Context, userID string) (model.NotificationSettings, bool, error) {
	settings := DefaultNotificationSettings(userID)
	var emailEnabled, telegramEnabled, notifyWarning, notifyCritical, notifyResolved int
	var quietHoursEnabled, webhookEnabled int
	var createdAt, updatedAt string
	var pausedUntil sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT email_enabled, telegram_enabled, telegram_chat_id, notify_warning, notify_critical, notify_resolved,
       memory_threshold_percent, disk_threshold_percent, packet_loss_percent, latency_threshold_ms,
       repeat_minutes, timezone, quiet_hours_enabled, quiet_hours_start, quiet_hours_end, alerts_paused_until,
       webhook_enabled, webhook_url, webhook_secret, created_at, updated_at
FROM notification_settings WHERE user_id = ?
`, userID).Scan(
		&emailEnabled, &telegramEnabled, &settings.TelegramChatID, &notifyWarning, &notifyCritical, &notifyResolved,
		&settings.MemoryThresholdPercent, &settings.DiskThresholdPercent, &settings.PacketLossPercent,
		&settings.LatencyThresholdMS, &settings.RepeatMinutes, &settings.Timezone, &quietHoursEnabled,
		&settings.QuietHoursStart, &settings.QuietHoursEnd, &pausedUntil, &webhookEnabled,
		&settings.WebhookURL, &settings.WebhookSecret, &createdAt, &updatedAt,
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
	settings.QuietHoursEnabled = quietHoursEnabled != 0
	settings.WebhookEnabled = webhookEnabled != 0
	settings.WebhookSecretConfigured = settings.WebhookSecret != ""
	if pausedUntil.Valid && pausedUntil.String != "" {
		value := parseTime(pausedUntil.String)
		settings.AlertsPausedUntil = &value
	}
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
  latency_threshold_ms, repeat_minutes, timezone, quiet_hours_enabled, quiet_hours_start, quiet_hours_end,
  alerts_paused_until, webhook_enabled, webhook_url, webhook_secret, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
  timezone = excluded.timezone,
  quiet_hours_enabled = excluded.quiet_hours_enabled,
  quiet_hours_start = excluded.quiet_hours_start,
  quiet_hours_end = excluded.quiet_hours_end,
  alerts_paused_until = excluded.alerts_paused_until,
  webhook_enabled = excluded.webhook_enabled,
  webhook_url = excluded.webhook_url,
  webhook_secret = CASE WHEN excluded.webhook_secret != '' THEN excluded.webhook_secret ELSE notification_settings.webhook_secret END,
  updated_at = excluded.updated_at
`, settings.UserID, boolInt(settings.EmailEnabled), boolInt(settings.TelegramEnabled), strings.TrimSpace(settings.TelegramChatID),
		boolInt(settings.NotifyWarning), boolInt(settings.NotifyCritical), boolInt(settings.NotifyResolved),
		settings.MemoryThresholdPercent, settings.DiskThresholdPercent, settings.PacketLossPercent,
		settings.LatencyThresholdMS, settings.RepeatMinutes, settings.Timezone, boolInt(settings.QuietHoursEnabled),
		settings.QuietHoursStart, settings.QuietHoursEnd, nullableTime(settings.AlertsPausedUntil),
		boolInt(settings.WebhookEnabled), strings.TrimSpace(settings.WebhookURL), strings.TrimSpace(settings.WebhookSecret), now, now)
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
	maxAttempts := delivery.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	var deviceID any
	if strings.TrimSpace(delivery.DeviceID) != "" {
		deviceID = delivery.DeviceID
	}
	res, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO notification_deliveries (
  id, user_id, device_id, alert_id, dedupe_key, event, channel, status, title, body,
  destination, destination_masked, error, attempt_count, max_attempts, created_at,
  next_attempt_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, 'queued', ?, ?, ?, ?, '', 0, ?, ?, ?, ?)
`, id, delivery.UserID, deviceID, delivery.AlertID, dedupeKey, delivery.Event, delivery.Channel,
		delivery.Title, delivery.Body, delivery.Destination, delivery.DestinationMasked, maxAttempts, now, now, now)
	if err != nil {
		return model.NotificationDelivery{}, false, err
	}
	inserted, _ := res.RowsAffected()
	if inserted == 0 {
		return model.NotificationDelivery{}, false, nil
	}
	delivery.ID = id
	delivery.Status = "queued"
	delivery.MaxAttempts = maxAttempts
	delivery.CreatedAt = parseTime(now)
	nextAttemptAt := parseTime(now)
	delivery.NextAttemptAt = &nextAttemptAt
	return delivery, true, nil
}

func (s *Store) CompleteNotificationDelivery(ctx context.Context, id, status, errorMessage string, nextAttemptAt *time.Time) error {
	status = strings.TrimSpace(status)
	if status != "sent" && status != "retry" && status != "dead_letter" {
		return errors.New("invalid notification delivery status")
	}
	if status == "retry" && nextAttemptAt == nil {
		return errors.New("retry notification delivery requires next attempt time")
	}
	now := nowText()
	var sentAt any
	var nextAttempt any
	if status == "sent" {
		sentAt = now
	} else if nextAttemptAt != nil {
		nextAttempt = notificationTimeText(nextAttemptAt.UTC())
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE notification_deliveries
SET status = ?, error = ?, sent_at = ?, next_attempt_at = COALESCE(?, ''),
    lease_expires_at = NULL, updated_at = ?
WHERE id = ? AND status = 'sending'
`, status, strings.TrimSpace(errorMessage), sentAt, nextAttempt, now, id)
	if err != nil {
		return err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return errors.New("notification delivery is not claimed")
	}
	return nil
}

func (s *Store) ClaimNotificationDelivery(ctx context.Context, id string, now time.Time, leaseDuration time.Duration) (model.NotificationDelivery, bool, error) {
	deliveries, err := s.claimNotificationDeliveries(ctx, strings.TrimSpace(id), now, leaseDuration, 1)
	if err != nil || len(deliveries) == 0 {
		return model.NotificationDelivery{}, false, err
	}
	return deliveries[0], true, nil
}

func (s *Store) ClaimNotificationDeliveries(ctx context.Context, now time.Time, leaseDuration time.Duration, limit int) ([]model.NotificationDelivery, error) {
	return s.claimNotificationDeliveries(ctx, "", now, leaseDuration, limit)
}

func (s *Store) claimNotificationDeliveries(ctx context.Context, onlyID string, now time.Time, leaseDuration time.Duration, limit int) ([]model.NotificationDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Minute
	}
	now = now.UTC()
	nowValue := notificationTimeText(now)
	leaseValue := notificationTimeText(now.Add(leaseDuration))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
UPDATE notification_deliveries
SET status = 'dead_letter', error = CASE WHEN error = '' THEN 'Попытки доставки исчерпаны. Проверьте настройки канала.' ELSE error END,
    next_attempt_at = '', lease_expires_at = NULL, updated_at = ?
WHERE status IN ('retry', 'sending') AND attempt_count >= max_attempts
  AND (status = 'retry' OR lease_expires_at IS NULL OR lease_expires_at <= ?)
`, nowValue, nowValue); err != nil {
		return nil, err
	}
	query := `
SELECT id
FROM notification_deliveries
WHERE attempt_count < max_attempts
  AND ((status IN ('queued', 'retry') AND (next_attempt_at = '' OR next_attempt_at <= ?))
       OR (status = 'sending' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?))`
	args := []any{nowValue, nowValue}
	if onlyID != "" {
		query += ` AND id = ?`
		args = append(args, onlyID)
	}
	query += ` ORDER BY created_at ASC LIMIT ?`
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	claimed := make([]model.NotificationDelivery, 0, len(ids))
	for _, id := range ids {
		res, err := tx.ExecContext(ctx, `
UPDATE notification_deliveries
SET status = 'sending', attempt_count = attempt_count + 1, last_attempt_at = ?,
    lease_expires_at = ?, next_attempt_at = '', updated_at = ?
WHERE id = ? AND attempt_count < max_attempts
  AND ((status IN ('queued', 'retry') AND (next_attempt_at = '' OR next_attempt_at <= ?))
       OR (status = 'sending' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?))
`, nowValue, leaseValue, nowValue, id, nowValue, nowValue)
		if err != nil {
			return nil, err
		}
		updated, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if updated == 0 {
			continue
		}
		delivery, err := scanNotificationDelivery(tx.QueryRowContext(ctx, notificationDeliverySelect+` WHERE id = ?`, id))
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, delivery)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return claimed, nil
}

func (s *Store) PurgeNotificationDeliveriesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM notification_deliveries
WHERE status IN ('sent', 'dead_letter', 'failed') AND updated_at < ?
`, notificationTimeText(cutoff.UTC()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

const notificationDeliverySelect = `
SELECT id, user_id, COALESCE(device_id, ''), alert_id, event, channel, status, title, body,
       destination, destination_masked, error, attempt_count, max_attempts, created_at,
       last_attempt_at, next_attempt_at, sent_at
FROM notification_deliveries`

func (s *Store) LatestNotificationDelivery(ctx context.Context, userID, alertID, channel string) (model.NotificationDelivery, bool, error) {
	row := s.db.QueryRowContext(ctx, notificationDeliverySelect+`
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
	query := `
SELECT nd.id, nd.user_id, COALESCE(nd.device_id, ''), nd.alert_id, nd.event, nd.channel, nd.status,
       nd.title, nd.body, nd.destination, nd.destination_masked, nd.error, nd.attempt_count,
       nd.max_attempts, nd.created_at, nd.last_attempt_at, nd.next_attempt_at, nd.sent_at,
       COALESCE(a.severity, '')
FROM notification_deliveries nd
LEFT JOIN alerts a ON a.id = nd.alert_id
WHERE nd.user_id = ?`
	args := []any{opts.UserID}
	if opts.DeviceID != "" {
		query += ` AND nd.device_id = ?`
		args = append(args, opts.DeviceID)
	}
	if opts.Severity != "" {
		query += ` AND a.severity = ?`
		args = append(args, opts.Severity)
	}
	if opts.Event != "" {
		query += ` AND nd.event = ?`
		args = append(args, opts.Event)
	}
	if opts.Channel != "" {
		query += ` AND nd.channel = ?`
		args = append(args, opts.Channel)
	}
	if opts.Status != "" {
		query += ` AND nd.status = ?`
		args = append(args, opts.Status)
	}
	query += ` ORDER BY nd.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	deliveries := make([]model.NotificationDelivery, 0)
	for rows.Next() {
		delivery, err := scanNotificationDeliveryWithSeverity(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

func (s *Store) NotificationDeliveryMetrics(ctx context.Context, userID string, now time.Time) (model.NotificationDeliveryMetrics, error) {
	metrics := model.NotificationDeliveryMetrics{
		Channels: make([]model.NotificationChannelMetrics, 0, 3),
	}
	var oldestQueued sql.NullString
	if err := s.db.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN status IN ('queued', 'sending') THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status IN ('retry', 'failed') THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'dead_letter' THEN 1 ELSE 0 END), 0),
  MIN(CASE WHEN status IN ('queued', 'sending', 'retry') THEN created_at END)
FROM notification_deliveries
WHERE user_id = ?
`, userID).Scan(&metrics.Queued, &metrics.Sent, &metrics.Failed, &metrics.DeadLetter, &oldestQueued); err != nil {
		return model.NotificationDeliveryMetrics{}, err
	}
	now = now.UTC()
	if oldestQueued.Valid && oldestQueued.String != "" {
		value := parseTime(oldestQueued.String)
		metrics.OldestQueuedAt = &value
		if age := now.Sub(value); age > 0 {
			metrics.OldestQueueAgeSeconds = int64(age / time.Second)
		}
	}
	for _, channel := range []string{"email", "telegram", "webhook"} {
		channelMetrics := model.NotificationChannelMetrics{Channel: channel}
		var lastSuccess sql.NullString
		err := s.db.QueryRowContext(ctx, `
SELECT sent_at
FROM notification_deliveries
WHERE user_id = ? AND channel = ? AND status = 'sent' AND sent_at IS NOT NULL
ORDER BY sent_at DESC LIMIT 1
`, userID, channel).Scan(&lastSuccess)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return model.NotificationDeliveryMetrics{}, err
		}
		if lastSuccess.Valid && lastSuccess.String != "" {
			value := parseTime(lastSuccess.String)
			channelMetrics.LastSuccessAt = &value
		}
		var lastErrorAt sql.NullString
		err = s.db.QueryRowContext(ctx, `
SELECT error, status, updated_at
FROM notification_deliveries
WHERE user_id = ? AND channel = ? AND status IN ('retry', 'failed', 'dead_letter')
ORDER BY updated_at DESC LIMIT 1
`, userID, channel).Scan(&channelMetrics.LastError, &channelMetrics.LastErrorStatus, &lastErrorAt)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return model.NotificationDeliveryMetrics{}, err
		}
		if lastErrorAt.Valid && lastErrorAt.String != "" {
			value := parseTime(lastErrorAt.String)
			channelMetrics.LastErrorAt = &value
		}
		metrics.Channels = append(metrics.Channels, channelMetrics)
	}
	return metrics, nil
}

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanNotificationDelivery(scanner notificationScanner) (model.NotificationDelivery, error) {
	var delivery model.NotificationDelivery
	var createdAt string
	var lastAttemptAt, nextAttemptAt, sentAt sql.NullString
	err := scanner.Scan(
		&delivery.ID, &delivery.UserID, &delivery.DeviceID, &delivery.AlertID, &delivery.Event,
		&delivery.Channel, &delivery.Status, &delivery.Title, &delivery.Body, &delivery.Destination,
		&delivery.DestinationMasked, &delivery.Error, &delivery.AttemptCount, &delivery.MaxAttempts,
		&createdAt, &lastAttemptAt, &nextAttemptAt, &sentAt,
	)
	if err != nil {
		return model.NotificationDelivery{}, err
	}
	delivery.CreatedAt = parseTime(createdAt)
	if lastAttemptAt.Valid && lastAttemptAt.String != "" {
		value := parseTime(lastAttemptAt.String)
		delivery.LastAttemptAt = &value
	}
	if nextAttemptAt.Valid && nextAttemptAt.String != "" {
		value := parseTime(nextAttemptAt.String)
		delivery.NextAttemptAt = &value
	}
	if sentAt.Valid {
		value := parseTime(sentAt.String)
		delivery.SentAt = &value
	}
	return delivery, nil
}

func scanNotificationDeliveryWithSeverity(scanner notificationScanner) (model.NotificationDelivery, error) {
	var delivery model.NotificationDelivery
	var createdAt string
	var lastAttemptAt, nextAttemptAt, sentAt sql.NullString
	err := scanner.Scan(
		&delivery.ID, &delivery.UserID, &delivery.DeviceID, &delivery.AlertID, &delivery.Event,
		&delivery.Channel, &delivery.Status, &delivery.Title, &delivery.Body, &delivery.Destination,
		&delivery.DestinationMasked, &delivery.Error, &delivery.AttemptCount, &delivery.MaxAttempts,
		&createdAt, &lastAttemptAt, &nextAttemptAt, &sentAt, &delivery.Severity,
	)
	if err != nil {
		return model.NotificationDelivery{}, err
	}
	delivery.CreatedAt = parseTime(createdAt)
	if lastAttemptAt.Valid && lastAttemptAt.String != "" {
		value := parseTime(lastAttemptAt.String)
		delivery.LastAttemptAt = &value
	}
	if nextAttemptAt.Valid && nextAttemptAt.String != "" {
		value := parseTime(nextAttemptAt.String)
		delivery.NextAttemptAt = &value
	}
	if sentAt.Valid && sentAt.String != "" {
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

func notificationTimeText(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return notificationTimeText(value.UTC())
}
