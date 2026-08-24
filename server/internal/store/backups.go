package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
)

type DeviceBackupMetadata struct {
	OpenWrtVersion string
	Target         string
	Model          string
	Manifest       json.RawMessage
}

var ErrDeviceBackupInProgress = errors.New("a device backup is already in progress")

func (s *Store) RequestDeviceBackup(ctx context.Context, deviceID string) (model.DeviceBackup, model.Command, bool, error) {
	command, err := s.newCommand(deviceID, "system_backup_create", json.RawMessage(`{}`), 24*time.Hour)
	if err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	backupID, err := randomID("bkp")
	if err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	defer tx.Rollback()
	var deviceExists, backupActive bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?), EXISTS(SELECT 1 FROM device_backups WHERE device_id = ? AND status = 'creating')`, deviceID, deviceID).Scan(&deviceExists, &backupActive); err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	if !deviceExists {
		return model.DeviceBackup{}, model.Command{}, false, nil
	}
	if backupActive {
		return model.DeviceBackup{}, model.Command{}, true, ErrDeviceBackupInProgress
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO commands (id, device_id, type, args_json, status, max_attempts, created_at, expires_at, nonce, signature_key_id, signature)
VALUES (?, ?, ?, ?, 'queued', ?, ?, ?, ?, ?, ?)
`, command.ID, command.DeviceID, command.Type, string(command.Args), command.MaxAttempts, command.CreatedAt.Format(time.RFC3339Nano), command.ExpiresAt.Format(time.RFC3339Nano), command.Nonce, command.SignatureKeyID, command.Signature); err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO device_backups (id, device_id, command_id, status, created_at) VALUES (?, ?, ?, 'creating', ?)`, backupID, deviceID, command.ID, command.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.DeviceBackup{}, model.Command{}, false, err
	}
	return model.DeviceBackup{ID: backupID, DeviceID: deviceID, CommandID: command.ID, Status: "creating", CreatedAt: command.CreatedAt}, command, true, nil
}

func (s *Store) SaveDeviceBackup(ctx context.Context, deviceID, commandID string, archive []byte, metadata DeviceBackupMetadata) (model.DeviceBackup, bool, error) {
	if s.sensitiveCipher == nil {
		return model.DeviceBackup{}, false, errors.New("backup encryption is unavailable")
	}
	encryptionContext := sensitiveContext("device_backup", deviceID+":"+commandID, "archive")
	encrypted, err := s.sensitiveCipher.EncryptBytesWithContext(encryptionContext, archive)
	if err != nil {
		return model.DeviceBackup{}, false, err
	}
	digest := sha256.Sum256(archive)
	manifest := NormalizeRawJSON(metadata.Manifest)
	if len(metadata.Manifest) == 0 {
		manifest = json.RawMessage(`[]`)
	}
	now := nowText()
	res, err := s.db.ExecContext(ctx, `
UPDATE device_backups
SET status = 'ready', archive_ciphertext = ?, size_bytes = ?, sha256 = ?, openwrt_version = ?,
    target = ?, model = ?, manifest_json = ?, error = '', completed_at = ?
WHERE device_id = ? AND command_id = ? AND status = 'creating'
`, encrypted, len(archive), hex.EncodeToString(digest[:]), strings.TrimSpace(metadata.OpenWrtVersion),
		strings.TrimSpace(metadata.Target), strings.TrimSpace(metadata.Model), string(manifest), now, deviceID, commandID)
	if err != nil {
		return model.DeviceBackup{}, false, err
	}
	updated, err := res.RowsAffected()
	if err != nil || updated == 0 {
		return model.DeviceBackup{}, false, err
	}
	backup, found, err := s.GetDeviceBackup(ctx, deviceID, commandID, true)
	return backup, found, err
}

func (s *Store) FailDeviceBackup(ctx context.Context, commandID, errorMessage string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE device_backups SET status = 'failed', error = ?, completed_at = ? WHERE command_id = ? AND status = 'creating'`, strings.TrimSpace(errorMessage), nowText(), commandID)
	return err
}

func (s *Store) ListDeviceBackups(ctx context.Context, deviceID string) ([]model.DeviceBackup, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil || !exists {
		return nil, exists, err
	}
	rows, err := s.db.QueryContext(ctx, backupSelect+` WHERE device_id = ? ORDER BY created_at DESC LIMIT 50`, deviceID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	backups := make([]model.DeviceBackup, 0)
	for rows.Next() {
		backup, err := scanDeviceBackup(rows)
		if err != nil {
			return nil, false, err
		}
		backups = append(backups, backup)
	}
	return backups, true, rows.Err()
}

func (s *Store) GetDeviceBackup(ctx context.Context, deviceID, idOrCommand string, allowCommandID bool) (model.DeviceBackup, bool, error) {
	query := backupSelect + ` WHERE device_id = ? AND id = ?`
	if allowCommandID {
		query = backupSelect + ` WHERE device_id = ? AND (id = ? OR command_id = ?)`
	}
	var row *sql.Row
	if allowCommandID {
		row = s.db.QueryRowContext(ctx, query, deviceID, idOrCommand, idOrCommand)
	} else {
		row = s.db.QueryRowContext(ctx, query, deviceID, idOrCommand)
	}
	backup, err := scanDeviceBackup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DeviceBackup{}, false, nil
	}
	return backup, err == nil, err
}

func (s *Store) DeviceBackupArchive(ctx context.Context, deviceID, backupID string) (model.DeviceBackup, []byte, bool, error) {
	backup, found, err := s.GetDeviceBackup(ctx, deviceID, backupID, false)
	if err != nil || !found || backup.Status != "ready" {
		return backup, nil, found, err
	}
	var encrypted []byte
	if err := s.db.QueryRowContext(ctx, `SELECT archive_ciphertext FROM device_backups WHERE id = ? AND device_id = ?`, backupID, deviceID).Scan(&encrypted); err != nil {
		return model.DeviceBackup{}, nil, false, err
	}
	encryptionContext := sensitiveContext("device_backup", deviceID+":"+backup.CommandID, "archive")
	archive, err := s.sensitiveCipher.DecryptBytesWithContext(encryptionContext, encrypted)
	return backup, archive, err == nil, err
}

func (s *Store) DeleteDeviceBackup(ctx context.Context, deviceID, backupID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM device_backups WHERE id = ? AND device_id = ?`, backupID, deviceID)
	if err != nil {
		return false, err
	}
	count, err := res.RowsAffected()
	return count == 1, err
}

func (s *Store) PurgeDeviceBackupsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM device_backups WHERE status IN ('ready', 'failed') AND created_at < ?`, notificationTimeText(cutoff.UTC()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CreateSQLiteSnapshot creates a consistent temporary database file. The
// caller owns the returned path and must remove it after streaming or copying.
func (s *Store) CreateSQLiteSnapshot(ctx context.Context) (string, error) {
	if s.dbPath == "" || s.dbPath == ":memory:" || strings.HasPrefix(s.dbPath, "file::memory:") {
		return "", errors.New("SQLite snapshot is unavailable for an in-memory database")
	}
	dir := filepath.Dir(s.dbPath)
	temporary, err := os.CreateTemp(dir, ".rmm-snapshot-*.db")
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	_ = os.Remove(path)
	escaped := strings.ReplaceAll(path, "'", "''")
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO '`+escaped+`'`); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("create consistent SQLite snapshot: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

const backupSelect = `SELECT id, device_id, command_id, status, size_bytes, sha256, openwrt_version, target, model, manifest_json, error, created_at, completed_at FROM device_backups`

func scanDeviceBackup(scanner scanner) (model.DeviceBackup, error) {
	var backup model.DeviceBackup
	var manifest, createdAt string
	var completedAt sql.NullString
	err := scanner.Scan(&backup.ID, &backup.DeviceID, &backup.CommandID, &backup.Status, &backup.SizeBytes,
		&backup.SHA256, &backup.OpenWrtVersion, &backup.Target, &backup.Model, &manifest, &backup.Error, &createdAt, &completedAt)
	if err != nil {
		return model.DeviceBackup{}, err
	}
	backup.Manifest = json.RawMessage(manifest)
	backup.CreatedAt = parseTime(createdAt)
	if completedAt.Valid {
		value := parseTime(completedAt.String)
		backup.CompletedAt = &value
	}
	return backup, nil
}
