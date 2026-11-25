package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"rmm-openwrt/server/internal/model"
)

var ErrRecoveryAccountChanged = errors.New("account credentials have changed")
var ErrRecoveryEmailInUse = errors.New("email is already in use")

// SQLite serializes writes on the store connection. PostgreSQL locks the
// account so a reset issued to a former address cannot survive an address change.
func lockRecoveryAccount(ctx context.Context, tx *sqliteTx, userID string) (string, string, error) {
	query := "SELECT email, password_hash FROM users WHERE id = ? AND disabled = 0"
	if tx.postgres {
		query += " FOR UPDATE"
	}
	var email, hash string
	err := tx.QueryRowContext(ctx, query, userID).Scan(&email, &hash)
	return email, hash, err
}

func checkRecoveryEmailAvailable(ctx context.Context, tx *sqliteTx, userID, email string) error {
	if email == "" {
		return nil
	}
	var used bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = ? COLLATE NOCASE AND id <> ?)`, email, userID).Scan(&used); err != nil {
		return err
	}
	if used {
		return ErrRecoveryEmailInUse
	}
	return nil
}

func (s *Store) BeginRecoveryEmailChange(ctx context.Context, userID, currentHash, currentEmail, email, codeHash string, expires time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, hash, err := lockRecoveryAccount(ctx, tx, userID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (old != currentEmail || hash != currentHash) {
		return ErrRecoveryAccountChanged
	}
	if err != nil {
		return err
	}
	if err := checkRecoveryEmailAvailable(ctx, tx, userID, email); err != nil {
		return err
	}
	destination, err := s.encryptSensitive(sensitiveContext("contact_verification", userID+":recovery_email", "destination"), email)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO contact_verifications (user_id, channel, destination, code_hash, expires_at, verified_at, created_at)
VALUES (?, 'recovery_email', ?, ?, ?, NULL, ?) ON CONFLICT(user_id, channel) DO UPDATE SET
destination = excluded.destination, code_hash = excluded.code_hash, expires_at = excluded.expires_at, created_at = excluded.created_at, verified_at = NULL`,
		userID, destination, codeHash, notificationTimeText(expires), nowText())
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ConfirmRecoveryEmailChange(ctx context.Context, userID, codeHash string) (model.User, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, false, err
	}
	defer tx.Rollback()
	old, _, err := lockRecoveryAccount(ctx, tx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	var encrypted string
	err = tx.QueryRowContext(ctx, `DELETE FROM contact_verifications WHERE user_id = ? AND channel = 'recovery_email'
AND code_hash = ? AND julianday(expires_at) > julianday(?) RETURNING destination`, userID, codeHash, nowText()).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	email, err := s.decryptSensitive(sensitiveContext("contact_verification", userID+":recovery_email", "destination"), encrypted)
	if err != nil {
		return model.User{}, false, err
	}
	if err := checkRecoveryEmailAvailable(ctx, tx, userID, email); err != nil {
		return model.User{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email = ?, updated_at = ? WHERE id = ?`, email, nowText(), userID); err != nil {
		return model.User{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ?`, userID); err != nil {
		return model.User{}, false, err
	}
	if old != "" {
		// Commit the security notice with the address change. The standard
		// delivery worker retries it even when account alert emails are disabled.
		_, _, err = s.createNotificationDelivery(ctx, tx, model.NotificationDelivery{
			UserID: userID, Channel: "email", Event: "recovery_email_changed", Destination: old, DestinationMasked: "•••",
			Title: "Recovery email changed", Body: "The recovery email for your OpenWrt RMM account has changed. If you did not make this change, contact your administrator.",
		}, "recovery_email_changed:"+codeHash)
		if err != nil {
			return model.User{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.User{}, false, err
	}
	return s.GetUserByID(ctx, userID)
}
