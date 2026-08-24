package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxManagedBackupSize = 32 << 20

const restoreConfirmationTimeout = 10 * time.Minute

type restoreGuard struct {
	EmergencyPath string    `json:"emergency_path"`
	CreatedAt     time.Time `json:"created_at"`
	Deadline      time.Time `json:"deadline"`
}

func systemBackupOperation(ctx context.Context, client *http.Client, cfg config, cmd command) (string, int, map[string]any) {
	switch cmd.Type {
	case "system_backup_create":
		return createSystemBackup(ctx, client, cfg, cmd.ID)
	case "system_backup_restore":
		return restoreSystemBackup(ctx, client, cfg, cmd.Args)
	default:
		return "unsupported backup operation\n", 2, commandMetadata()
	}
}

func createSystemBackup(ctx context.Context, client *http.Client, cfg config, commandID string) (string, int, map[string]any) {
	if err := os.MkdirAll(cfg.BackupDir, 0o700); err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	file, err := os.CreateTemp(cfg.BackupDir, ".rmm-system-backup-*.tar.gz")
	if err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Remove(path)
	defer os.Remove(path)
	output, code := execCommand(ctx, 90*time.Second, "sysupgrade", "-b", path)
	if code != 0 {
		return output, code, commandMetadata()
	}
	archive, err := readLimitedFile(path, maxManagedBackupSize)
	if err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.ServerURL+"/api/agent/backups/"+commandID, bytes.NewReader(archive))
	if err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	request.Header.Set("Authorization", "Bearer "+cfg.DeviceToken)
	request.Header.Set("Content-Type", "application/gzip")
	request.Header.Set("X-RMM-Device-ID", cfg.DeviceID)
	request.Header.Set("X-RMM-OpenWrt-Version", openwrtVersion())
	request.Header.Set("X-RMM-Target", openwrtTarget())
	request.Header.Set("X-RMM-Model", boardModel())
	uploadClient := *client
	uploadClient.Timeout = 2 * time.Minute
	response, err := uploadClient.Do(request)
	if err != nil {
		return fmt.Sprintf("backup upload failed: %v\n", err), 1, commandMetadata()
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Sprintf("backup upload failed with HTTP %d: %s\n", response.StatusCode, strings.TrimSpace(string(responseBody))), 1, commandMetadata()
	}
	var stored map[string]any
	_ = json.Unmarshal(responseBody, &stored)
	return fmt.Sprintf("system backup uploaded securely (%d bytes)\n", len(archive)), 0, stored
}

func restoreSystemBackup(ctx context.Context, client *http.Client, cfg config, raw json.RawMessage) (string, int, map[string]any) {
	args := map[string]string{}
	if json.Unmarshal(raw, &args) != nil || !safeSessionID(args["backup_id"]) || len(args["sha256"]) != 64 || args["target"] == "" {
		return "backup restore arguments are invalid\n", 2, commandMetadata()
	}
	if _, pending, err := readRestoreGuard(cfg.RecoveryDir); err != nil {
		return "existing restore guard is unreadable; restore cancelled: " + err.Error() + "\n", 1, commandMetadata()
	} else if pending {
		return "a previous restore is still waiting for cloud confirmation\n", 2, commandMetadata()
	}
	if args["target"] != openwrtTarget() {
		return "backup target does not match this router\n", 2, commandMetadata()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ServerURL+"/api/agent/backups/"+args["backup_id"], nil)
	if err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	request.Header.Set("Authorization", "Bearer "+cfg.DeviceToken)
	request.Header.Set("X-RMM-Device-ID", cfg.DeviceID)
	downloadClient := *client
	downloadClient.Timeout = 2 * time.Minute
	response, err := downloadClient.Do(request)
	if err != nil {
		return fmt.Sprintf("backup download failed: %v\n", err), 1, commandMetadata()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Sprintf("backup download failed with HTTP %d\n", response.StatusCode), 1, commandMetadata()
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, maxManagedBackupSize+1))
	if err != nil || len(archive) == 0 || len(archive) > maxManagedBackupSize {
		return "downloaded backup is empty or too large\n", 1, commandMetadata()
	}
	digest := sha256.Sum256(archive)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), args["sha256"]) {
		return "downloaded backup checksum does not match\n", 1, commandMetadata()
	}
	if err := os.MkdirAll(cfg.BackupDir, 0o700); err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	archivePath := filepath.Join(cfg.BackupDir, ".rmm-restore-"+args["backup_id"]+".tar.gz")
	if err := os.MkdirAll(cfg.RecoveryDir, 0o700); err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	emergencyPath := filepath.Join(cfg.RecoveryDir, "emergency-before-restore.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		return err.Error() + "\n", 1, commandMetadata()
	}
	defer os.Remove(archivePath)
	if output, code := execCommand(ctx, 90*time.Second, "sysupgrade", "-b", emergencyPath); code != 0 {
		return "emergency backup failed; restore cancelled\n" + output, code, commandMetadata()
	}
	if err := os.Chmod(emergencyPath, 0o600); err != nil {
		return "emergency backup permissions are unsafe; restore cancelled\n", 1, commandMetadata()
	}
	now := time.Now().UTC()
	if err := writeRestoreGuard(cfg.RecoveryDir, restoreGuard{EmergencyPath: emergencyPath, CreatedAt: now, Deadline: now.Add(restoreConfirmationTimeout)}); err != nil {
		return "failed to create restore guard; restore cancelled: " + err.Error() + "\n", 1, commandMetadata()
	}
	output, code := execCommand(ctx, 2*time.Minute, "sysupgrade", "-r", archivePath)
	if code != 0 {
		return output, code, commandMetadata()
	}
	// Keep the current cloud identity and pinned signing key even when restoring
	// an archive created before a credential rotation.
	if err := saveConfig(cfg); err != nil {
		return output + "\nrestored configuration but failed to preserve agent identity: " + err.Error() + "\n", 1, commandMetadata()
	}
	return output + "\nconfiguration restored; waiting for cloud confirmation before committing the restore\n", 0, map[string]any{"backup_id": args["backup_id"], "emergency_backup": emergencyPath, "guard_timeout_seconds": int(restoreConfirmationTimeout.Seconds())}
}

func restoreGuardPath(dir string) string {
	return filepath.Join(dir, "pending-restore.json")
}

func writeRestoreGuard(dir string, guard restoreGuard) error {
	if strings.TrimSpace(guard.EmergencyPath) == "" || guard.CreatedAt.IsZero() || !guard.Deadline.After(guard.CreatedAt) {
		return errors.New("restore guard is invalid")
	}
	data, err := json.Marshal(guard)
	if err != nil {
		return err
	}
	path := restoreGuardPath(dir)
	temporary := path + ".new"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func readRestoreGuard(dir string) (restoreGuard, bool, error) {
	if strings.TrimSpace(dir) == "" {
		return restoreGuard{}, false, nil
	}
	data, err := os.ReadFile(restoreGuardPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return restoreGuard{}, false, nil
	}
	if err != nil {
		return restoreGuard{}, false, err
	}
	var guard restoreGuard
	if err := json.Unmarshal(data, &guard); err != nil {
		return restoreGuard{}, false, err
	}
	if strings.TrimSpace(guard.EmergencyPath) == "" || guard.CreatedAt.IsZero() || !guard.Deadline.After(guard.CreatedAt) {
		return restoreGuard{}, false, errors.New("restore guard is invalid")
	}
	return guard, true, nil
}

func confirmPendingRestore(dir string, heartbeatStartedAt time.Time) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	guard, found, err := readRestoreGuard(dir)
	if err != nil || !found || !heartbeatStartedAt.After(guard.CreatedAt) {
		return err
	}
	return os.Remove(restoreGuardPath(dir))
}

func rollbackPendingRestore(ctx context.Context, cfg config, now time.Time) (bool, error) {
	if strings.TrimSpace(cfg.RecoveryDir) == "" {
		return false, nil
	}
	guard, found, err := readRestoreGuard(cfg.RecoveryDir)
	if err != nil || !found || now.Before(guard.Deadline) {
		return false, err
	}
	info, err := os.Stat(guard.EmergencyPath)
	if err != nil || !info.Mode().IsRegular() {
		return false, errors.New("emergency restore archive is unavailable")
	}
	output, code := execCommand(ctx, 2*time.Minute, "sysupgrade", "-r", guard.EmergencyPath)
	if code != 0 {
		return false, fmt.Errorf("sysupgrade rollback failed with code %d: %s", code, strings.TrimSpace(output))
	}
	if err := saveConfig(cfg); err != nil {
		return false, fmt.Errorf("preserve agent identity after rollback: %w", err)
	}
	if err := os.Remove(restoreGuardPath(cfg.RecoveryDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, errors.New("system backup is empty or exceeds 32 MiB")
	}
	return data, nil
}

func boardModel() string {
	var board struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal([]byte(commandOutput("ubus", "call", "system", "board")), &board)
	return strings.TrimSpace(board.Model)
}
