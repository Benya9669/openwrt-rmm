package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"rmm-openwrt/server/internal/authn"
)

func VerificationCodeHash(code string) (string, error) {
	return authn.HashPassword("verification:" + strings.TrimSpace(code))
}

func (s *Store) BeginContactVerification(ctx context.Context, userID, channel, destination, codeHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO contact_verifications (user_id, channel, destination, code_hash, expires_at, verified_at, created_at)
VALUES (?, ?, ?, ?, ?, NULL, ?)
ON CONFLICT(user_id, channel) DO UPDATE SET
  destination = excluded.destination, code_hash = excluded.code_hash,
  expires_at = excluded.expires_at, verified_at = NULL, created_at = excluded.created_at
`, userID, channel, destination, codeHash, notificationTimeText(expiresAt), nowText())
	return err
}

func (s *Store) ConfirmContactVerification(ctx context.Context, userID, channel, code string) (string, bool, error) {
	var destination, expected, expires string
	err := s.db.QueryRowContext(ctx, `
SELECT destination, code_hash, expires_at FROM contact_verifications
WHERE user_id = ? AND channel = ? AND verified_at IS NULL
`, userID, channel).Scan(&destination, &expected, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if time.Now().UTC().After(parseTime(expires)) ||
		!authn.VerifyPassword(expected, "verification:"+strings.TrimSpace(code)) {
		return "", false, nil
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE contact_verifications SET verified_at = ? WHERE user_id = ? AND channel = ?
`, nowText(), userID, channel)
	return destination, err == nil, err
}

func (s *Store) ContactVerified(ctx context.Context, userID, channel, destination string) (bool, error) {
	var verified bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM contact_verifications
  WHERE user_id = ? AND channel = ? AND destination = ? AND verified_at IS NOT NULL
)
`, userID, channel, strings.TrimSpace(destination)).Scan(&verified)
	return verified, err
}
