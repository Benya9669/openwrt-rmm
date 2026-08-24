package store

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rmm-openwrt/internal/fieldcrypto"
)

func TestSensitiveDataEncryptionAndDeviceCredentialRotation(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "security.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.db.ExecContext(ctx, `INSERT INTO users (id, username, password_hash, role, created_at, updated_at) VALUES ('user-1', 'user1', 'hash', 'user', ?, ?)`, nowText(), nowText()); err != nil {
		t.Fatal(err)
	}
	settings := DefaultNotificationSettings("user-1")
	settings.TelegramChatID = "123456"
	settings.WebhookURL = "https://hooks.example.test/private"
	settings.WebhookSecret = "secret-value"
	if _, err := st.UpsertNotificationSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	cipher, err := fieldcrypto.New(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	if err := st.MigrateSensitiveData(ctx); err != nil {
		t.Fatal(err)
	}
	var rawSecret string
	if err := st.db.QueryRowContext(ctx, `SELECT webhook_secret FROM notification_settings WHERE user_id = 'user-1'`).Scan(&rawSecret); err != nil {
		t.Fatal(err)
	}
	if rawSecret == "secret-value" || !strings.HasPrefix(rawSecret, "enc:v1:") {
		t.Fatalf("webhook secret was not encrypted: %q", rawSecret)
	}
	loaded, _, err := st.GetNotificationSettings(ctx, "user-1")
	if err != nil || loaded.WebhookSecret != "secret-value" || loaded.TelegramChatID != "123456" {
		t.Fatalf("encrypted settings did not round-trip: %#v %v", loaded, err)
	}

	enrolled, err := st.EnrollDevice(ctx, "router", "25.12.4")
	if err != nil {
		t.Fatal(err)
	}
	epoch, rotated, err := st.RotateDeviceCredential(ctx, enrolled.DeviceID)
	if err != nil || !rotated || epoch != 2 {
		t.Fatalf("credential rotation failed: epoch=%d rotated=%v err=%v", epoch, rotated, err)
	}
	nextToken, pendingEpoch, err := st.PendingDeviceCredential(ctx, enrolled.DeviceID)
	if err != nil || nextToken == "" || pendingEpoch != 2 {
		t.Fatalf("pending credential is unavailable: epoch=%d err=%v", pendingEpoch, err)
	}
	if allowed, _ := st.AuthorizeDevice(ctx, enrolled.DeviceID, enrolled.DeviceToken); !allowed {
		t.Fatal("old credential stopped working before acknowledgement")
	}
	if allowed, _ := st.AuthorizeDevice(ctx, enrolled.DeviceID, nextToken); !allowed {
		t.Fatal("new credential was not accepted")
	}
	if pending, _, err := st.PendingDeviceCredential(ctx, enrolled.DeviceID); err != nil || pending != "" {
		t.Fatalf("acknowledged credential remained pending: %q %v", pending, err)
	}
}

func TestEncryptedDeviceBackupAndSQLiteSnapshot(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "backups.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cipher, _ := fieldcrypto.New(bytes.Repeat([]byte{5}, 32))
	_ = st.SetSensitiveDataCipher(cipher)
	enrolled, err := st.EnrollDevice(ctx, "router", "25.12.4")
	if err != nil {
		t.Fatal(err)
	}
	record, command, found, err := st.RequestDeviceBackup(ctx, enrolled.DeviceID)
	if err != nil || !found {
		t.Fatalf("request backup: found=%v err=%v", found, err)
	}
	if _, _, _, err := st.RequestDeviceBackup(ctx, enrolled.DeviceID); !errors.Is(err, ErrDeviceBackupInProgress) {
		t.Fatalf("parallel backup request was not rejected: %v", err)
	}
	archive := []byte("test-backup-archive")
	stored, found, err := st.SaveDeviceBackup(ctx, enrolled.DeviceID, command.ID, archive, DeviceBackupMetadata{Target: "ramips-mt7621", Manifest: json.RawMessage(`["etc/config/network"]`)})
	if err != nil || !found || stored.ID != record.ID {
		t.Fatalf("save backup failed: %#v %v", stored, err)
	}
	var raw []byte
	if err := st.db.QueryRowContext(ctx, `SELECT archive_ciphertext FROM device_backups WHERE id = ?`, record.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, archive) {
		t.Fatal("backup archive was stored in plaintext")
	}
	_, restored, found, err := st.DeviceBackupArchive(ctx, enrolled.DeviceID, record.ID)
	if err != nil || !found || !bytes.Equal(restored, archive) {
		t.Fatalf("backup did not round-trip: %q %v", restored, err)
	}
	snapshotPath, err := st.CreateSQLiteSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(snapshotPath)
	snapshot, err := os.ReadFile(snapshotPath)
	if err != nil || len(snapshot) < 512 || !bytes.HasPrefix(snapshot, []byte("SQLite format 3")) {
		t.Fatalf("invalid SQLite snapshot: %d bytes, %v", len(snapshot), err)
	}
}

func TestPersistentSecurityKeysArePinnedToDatabase(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "key-pinning.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, commandKey, _ := ed25519.GenerateKey(rand.Reader)
	if err := st.SetCommandSigningKey(commandKey); err != nil {
		t.Fatal(err)
	}
	_, wrongCommandKey, _ := ed25519.GenerateKey(rand.Reader)
	if err := st.SetCommandSigningKey(wrongCommandKey); err == nil {
		t.Fatal("replacement command signing key was accepted")
	}
	cipher, _ := fieldcrypto.New(bytes.Repeat([]byte{7}, 32))
	if err := st.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	wrongCipher, _ := fieldcrypto.New(bytes.Repeat([]byte{8}, 32))
	if err := st.SetSensitiveDataCipher(wrongCipher); err == nil {
		t.Fatal("replacement data encryption key was accepted")
	}
}
