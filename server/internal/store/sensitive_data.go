package store

import (
	"context"
	"fmt"
)

// MigrateSensitiveData encrypts legacy plaintext values in place after the
// persistent server key has been loaded. Encrypt is idempotent for enc:v1 data.
func (s *Store) MigrateSensitiveData(ctx context.Context) error {
	if s.sensitiveCipher == nil {
		return fmt.Errorf("sensitive data cipher is not configured")
	}
	type settingsRow struct{ id, telegram, webhookURL, webhookSecret string }
	settings := make([]settingsRow, 0)
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, telegram_chat_id, webhook_url, webhook_secret FROM notification_settings`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item settingsRow
		if err := rows.Scan(&item.id, &item.telegram, &item.webhookURL, &item.webhookSecret); err != nil {
			rows.Close()
			return err
		}
		settings = append(settings, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range settings {
		telegram, err := s.encryptSensitive(sensitiveContext("notification_settings", item.id, "telegram_chat_id"), item.telegram)
		if err != nil {
			return err
		}
		webhookURL, err := s.encryptSensitive(sensitiveContext("notification_settings", item.id, "webhook_url"), item.webhookURL)
		if err != nil {
			return err
		}
		webhookSecret, err := s.encryptSensitive(sensitiveContext("notification_settings", item.id, "webhook_secret"), item.webhookSecret)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE notification_settings SET telegram_chat_id = ?, webhook_url = ?, webhook_secret = ? WHERE user_id = ?`, telegram, webhookURL, webhookSecret, item.id); err != nil {
			return err
		}
	}

	type deliveryRow struct{ id, title, body, destination string }
	deliveries := make([]deliveryRow, 0)
	rows, err = s.db.QueryContext(ctx, `SELECT id, title, body, destination FROM notification_deliveries`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item deliveryRow
		if err := rows.Scan(&item.id, &item.title, &item.body, &item.destination); err != nil {
			rows.Close()
			return err
		}
		deliveries = append(deliveries, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range deliveries {
		title, err := s.encryptSensitive(sensitiveContext("notification_delivery", item.id, "title"), item.title)
		if err != nil {
			return err
		}
		body, err := s.encryptSensitive(sensitiveContext("notification_delivery", item.id, "body"), item.body)
		if err != nil {
			return err
		}
		destination, err := s.encryptSensitive(sensitiveContext("notification_delivery", item.id, "destination"), item.destination)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE notification_deliveries SET title = ?, body = ?, destination = ? WHERE id = ?`, title, body, destination, item.id); err != nil {
			return err
		}
	}

	type verificationRow struct{ userID, channel, destination string }
	verifications := make([]verificationRow, 0)
	rows, err = s.db.QueryContext(ctx, `SELECT user_id, channel, destination FROM contact_verifications`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item verificationRow
		if err := rows.Scan(&item.userID, &item.channel, &item.destination); err != nil {
			rows.Close()
			return err
		}
		verifications = append(verifications, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range verifications {
		destination, err := s.encryptSensitive(sensitiveContext("contact_verification", item.userID+":"+item.channel, "destination"), item.destination)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE contact_verifications SET destination = ? WHERE user_id = ? AND channel = ?`, destination, item.userID, item.channel); err != nil {
			return err
		}
	}
	return nil
}
