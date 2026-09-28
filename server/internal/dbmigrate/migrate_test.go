package dbmigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rmm-openwrt/internal/commandsig"
	"rmm-openwrt/internal/fieldcrypto"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func TestDryRunCurrentSQLiteSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rmm.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnsureBootstrapUser(ctx, "Admin", "test-password-hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollDevice(ctx, "router", "24.10"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceSHA256 != hash || report.Committed || len(report.Tables) < 20 {
		t.Fatalf("unexpected migration report: %+v", report)
	}
	if after, err := fileSHA256(path); err != nil || after != hash {
		t.Fatalf("dry run modified source: hash %s, error %v", after, err)
	}
}

func TestDryRunRejectsInvalidJSON(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rmm.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	device, err := st.EnrollDevice(ctx, "router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE devices SET inventory_json = ? WHERE id = ?", "{broken", device.DeviceID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestMigrationGuards(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rmm.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := sha256.Sum256([]byte("wrong"))
	_, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hex.EncodeToString(bad[:]), DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected hash guard, got %v", err)
	}
	_, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, TargetPostgres: "postgres://user:secret@host/db?sslmode=verify-full"})
	if err == nil || !strings.Contains(err.Error(), "confirm-cutover") {
		t.Fatalf("expected confirmation guard, got %v", err)
	}
	_, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, TargetPostgres: "postgres://user:secret@host/db?sslmode=disable", ConfirmCutover: true, DataEncryptionKey: "example", CommandSigningKey: "example"})
	if err == nil || !strings.Contains(err.Error(), "sslmode=verify-full") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected TLS guard without credential leak, got %v", err)
	}
}

func TestDryRunRejectsMismatchedRecoveryKey(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "rmm.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	signingPath := filepath.Join(dir, "signing.pem")
	signingKey, err := commandsig.LoadOrCreatePrivateKey(signingPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetCommandSigningKey(signingKey); err != nil {
		t.Fatal(err)
	}
	correctKey, err := fieldcrypto.LoadOrCreate(filepath.Join(dir, "correct.key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSensitiveDataCipher(correctKey); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	wrongPath := filepath.Join(dir, "wrong.key")
	if _, err := fieldcrypto.LoadOrCreate(wrongPath); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true, DataEncryptionKey: wrongPath, CommandSigningKey: signingPath})
	if err == nil || !strings.Contains(err.Error(), "data_encryption_key_id") {
		t.Fatalf("expected recovery key mismatch, got %v", err)
	}
}

func TestDryRunRejectsActiveWAL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rmm.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+"-wal", []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "non-empty WAL") {
		t.Fatalf("expected active WAL guard, got %v", err)
	}
}

func TestSnapshotSQLiteWithWAL(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.db")
	live, err := sql.Open("sqlite", livePath)
	if err != nil {
		t.Fatal(err)
	}
	live.SetMaxOpenConns(1)
	if _, err := live.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if _, err := live.ExecContext(ctx, "CREATE TABLE example (value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := live.ExecContext(ctx, "INSERT INTO example VALUES ('committed in WAL')"); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(dir, "stopped.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		from, err := os.Open(livePath + suffix)
		if err != nil {
			if suffix == "-shm" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatal(err)
		}
		to, err := os.Create(sourcePath + suffix)
		if err != nil {
			from.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(to, from)
		from.Close()
		to.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	beforeDB, err := fileSHA256(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	beforeWAL, err := fileSHA256(sourcePath + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	backupPath, hash, err := snapshotSQLiteWithWAL(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if after, err := fileSHA256(sourcePath); err != nil || after != beforeDB {
		t.Fatalf("source database changed: %v", err)
	}
	if after, err := fileSHA256(sourcePath + "-wal"); err != nil || after != beforeWAL {
		t.Fatalf("source WAL changed: %v", err)
	}
	if backupHash, err := fileSHA256(backupPath); err != nil || backupHash != hash {
		t.Fatalf("snapshot hash mismatch: %v", err)
	}
	if err := checkSourceFile(backupPath); err != nil {
		t.Fatal(err)
	}
	snapshot, err := sql.Open("sqlite", readOnlySQLiteURI(backupPath))
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	var value string
	if err := snapshot.QueryRowContext(ctx, "SELECT value FROM example").Scan(&value); err != nil || value != "committed in WAL" {
		t.Fatalf("snapshot missed committed WAL data: value=%q err=%v", value, err)
	}
}

func TestConvertValueRejectsMalformedSourceData(t *testing.T) {
	for _, item := range []struct {
		value  any
		column columnSpec
	}{
		{int64(2), columnSpec{name: "enabled", dataType: "SMALLINT", notNull: true}},
		{"{broken", columnSpec{name: "metrics_json", dataType: "JSONB", notNull: true}},
		{"not-a-time", columnSpec{name: "created_at", dataType: "TIMESTAMPTZ", notNull: true}},
		{nil, columnSpec{name: "created_at", dataType: "TIMESTAMPTZ", notNull: true}},
	} {
		if _, err := convertValue(item.value, item.column); err == nil {
			t.Fatalf("expected conversion error for %s", item.column.name)
		}
	}
}

func TestImportPostgresIntegration(t *testing.T) {
	targetURL := os.Getenv("RMM_TEST_POSTGRES_URL")
	if targetURL == "" {
		t.Skip("set RMM_TEST_POSTGRES_URL to an empty local PostgreSQL database")
	}
	u, err := url.Parse(targetURL)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Query().Get("sslmode") != "disable" {
		t.Fatal("integration test requires a local PostgreSQL URL with sslmode=disable")
	}
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "rmm.db")
	dataKeyPath := filepath.Join(dir, "data-encryption.key")
	signingKeyPath := filepath.Join(dir, "command-signing-ed25519.pem")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := commandsig.LoadOrCreatePrivateKey(signingKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetCommandSigningKey(privateKey); err != nil {
		t.Fatal(err)
	}
	cipher, err := fieldcrypto.LoadOrCreate(dataKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	user, err := st.EnsureBootstrapUser(ctx, "Admin", "test-password-hash")
	if err != nil {
		t.Fatal(err)
	}
	device, err := st.EnrollDevice(ctx, "router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	command, _, err := st.CreateCommand(ctx, device.DeviceID, "system.info", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	encryptedContact, err := cipher.EncryptWithContext("notification_settings\x00"+user.ID+"\x00telegram_chat_id", "123456")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.ExecContext(ctx, "INSERT INTO notification_settings (user_id, telegram_chat_id, created_at, updated_at) VALUES (?, ?, ?, ?)", user.ID, encryptedContact, now, now); err != nil {
		t.Fatal(err)
	}
	encryptedArchive, err := cipher.EncryptBytesWithContext("device_backup\x00"+device.DeviceID+":"+command.ID+"\x00archive", []byte("test archive"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.ExecContext(ctx, "INSERT INTO device_backups (id, device_id, command_id, status, archive_ciphertext, created_at) VALUES (?, ?, ?, ?, ?, ?)", "backup-1", device.DeviceID, command.ID, "ready", encryptedArchive, now); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.ExecContext(ctx, "INSERT INTO metric_samples (id, device_id, inventory_json, metrics_json, created_at) VALUES (?, ?, ?, ?, ?)", "metric-1", device.DeviceID, `{"host":"роутер"}`, `{"cpu":12}`, now); err != nil {
		t.Fatal(err)
	}
	if err := sqlite.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := AutoImport(ctx, Options{
		SourceSQLite:       path,
		TargetPostgres:     targetURL,
		DataEncryptionKey:  dataKeyPath,
		CommandSigningKey:  signingKeyPath,
		AllowInsecureLocal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !imported {
		t.Fatal("automatic import skipped an empty PostgreSQL database")
	}
	backupHash, err := fileSHA256(path + ".pre-postgres-" + hash[:12] + ".db")
	if err != nil || backupHash != hash {
		t.Fatalf("automatic SQLite recovery copy: hash=%s err=%v", backupHash, err)
	}
	postgres, err := sql.Open("pgx", targetURL)
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	var devices, commands, backups, metrics int
	if err := postgres.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices").Scan(&devices); err != nil {
		t.Fatal(err)
	}
	if err := postgres.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands").Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err := postgres.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_backups").Scan(&backups); err != nil {
		t.Fatal(err)
	}
	if err := postgres.QueryRowContext(ctx, "SELECT COUNT(*) FROM metric_samples").Scan(&metrics); err != nil {
		t.Fatal(err)
	}
	if devices != 1 || commands != 1 || backups != 1 || metrics != 1 {
		t.Fatalf("unexpected PostgreSQL counts: devices=%d commands=%d backups=%d metrics=%d", devices, commands, backups, metrics)
	}
	var importedArchive []byte
	if err := postgres.QueryRowContext(ctx, "SELECT archive_ciphertext FROM device_backups WHERE id='backup-1'").Scan(&importedArchive); err != nil {
		t.Fatal(err)
	}
	if string(importedArchive) != string(encryptedArchive) {
		t.Fatal("encrypted archive changed during import")
	}
	if err := EnsurePostgres(ctx, postgres); err != nil {
		t.Fatalf("automatic schema check after import: %v", err)
	}
	if imported, err := AutoImport(ctx, Options{SourceSQLite: path, TargetPostgres: targetURL, AllowInsecureLocal: true}); err != nil || imported {
		t.Fatalf("automatic import repeated on a populated target: imported=%v err=%v", imported, err)
	}
	pgStore, err := store.OpenPostgres(ctx, targetURL, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer pgStore.Close()
	if err := pgStore.SetCommandSigningKey(privateKey); err != nil {
		t.Fatal(err)
	}
	if err := pgStore.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	if _, _, found, err := pgStore.GetUserByUsername(ctx, "admin"); err != nil || !found {
		t.Fatalf("PostgreSQL account lookup: found=%v err=%v", found, err)
	}
	if _, found, err := pgStore.GetDevice(ctx, device.DeviceID); err != nil || !found {
		t.Fatalf("PostgreSQL device lookup: found=%v err=%v", found, err)
	}
	importedCommand, found, err := pgStore.ClaimNextCommand(ctx, device.DeviceID)
	if err != nil || !found {
		t.Fatalf("PostgreSQL command claim: found=%v err=%v", found, err)
	}
	newCommand, created, err := pgStore.CreateCommand(ctx, device.DeviceID, "system.info", []byte(`{}`))
	if err != nil || !created {
		t.Fatalf("PostgreSQL command creation: created=%v err=%v", created, err)
	}
	claimedCommand, found, err := pgStore.ClaimNextCommand(ctx, device.DeviceID)
	if err != nil || !found || claimedCommand.ID != newCommand.ID {
		t.Fatalf("PostgreSQL new command claim: found=%v err=%v", found, err)
	}
	for _, signed := range []model.Command{importedCommand, claimedCommand} {
		if signed.ExpiresAt == nil {
			t.Fatalf("PostgreSQL command %s has no expiration", signed.ID)
		}
		if err := commandsig.Verify(pgStore.CommandSigningPublicKey(), commandsig.Envelope{
			ID: signed.ID, DeviceID: signed.DeviceID, Type: signed.Type,
			Args: signed.Args, CreatedAt: signed.CreatedAt,
			ExpiresAt: *signed.ExpiresAt, Nonce: signed.Nonce,
		}, signed.Signature); err != nil {
			t.Fatalf("PostgreSQL command %s signature changed after round trip: %v", signed.ID, err)
		}
	}
	if _, err := pgStore.SaveHeartbeat(ctx, device.DeviceID, []byte(`{"host":"роутер"}`), []byte(`{"cpu":15}`)); err != nil {
		t.Fatalf("PostgreSQL heartbeat: %v", err)
	}
	if _, found, err := pgStore.GetNotificationSettings(ctx, user.ID); err != nil || !found {
		t.Fatalf("PostgreSQL notification settings: found=%v err=%v", found, err)
	}
	if archiveInfo, plain, found, err := pgStore.DeviceBackupArchive(ctx, device.DeviceID, "backup-1"); err != nil || !found || archiveInfo.ID != "backup-1" || string(plain) != "test archive" {
		t.Fatalf("PostgreSQL encrypted backup: found=%v err=%v", found, err)
	}
	secondUser, err := pgStore.CreateUser(ctx, "Second", "Second User", "second@example.test", "test-hash", "user")
	if err != nil {
		t.Fatalf("PostgreSQL create user: %v", err)
	}
	if _, _, found, err := pgStore.GetUserByUsername(ctx, "SECOND"); err != nil || !found {
		t.Fatalf("PostgreSQL case-insensitive user lookup: found=%v err=%v", found, err)
	}
	disabled := true
	if updated, found, err := pgStore.UpdateUserSecurity(ctx, secondUser.ID, &disabled, "", ""); err != nil || !found || !updated.Disabled {
		t.Fatalf("PostgreSQL user disable: found=%v err=%v", found, err)
	}
	disabled = false
	if updated, found, err := pgStore.UpdateUserSecurity(ctx, secondUser.ID, &disabled, "", ""); err != nil || !found || updated.Disabled {
		t.Fatalf("PostgreSQL user enable: found=%v err=%v", found, err)
	}
	settings := store.DefaultNotificationSettings(user.ID)
	settings.EmailEnabled = true
	if _, err := pgStore.UpsertNotificationSettings(ctx, settings); err != nil {
		t.Fatalf("PostgreSQL notification preferences: %v", err)
	}
	delivery, inserted, err := pgStore.CreateNotificationDelivery(ctx, model.NotificationDelivery{
		UserID: user.ID, Event: "test", Channel: "email", Title: "test", Body: "body",
		Destination: "owner@example.test", DestinationMasked: "o***@example.test", MaxAttempts: 3,
	}, "postgres-test")
	if err != nil || !inserted {
		t.Fatalf("PostgreSQL notification insert: inserted=%v err=%v", inserted, err)
	}
	if _, found, err := pgStore.ClaimNotificationDelivery(ctx, delivery.ID, time.Now().UTC().Add(time.Second), time.Minute); err != nil || !found {
		t.Fatalf("PostgreSQL notification claim: found=%v err=%v", found, err)
	}
	if err := pgStore.CompleteNotificationDelivery(ctx, delivery.ID, "sent", "", nil); err != nil {
		t.Fatalf("PostgreSQL notification completion: %v", err)
	}
	if _, err := pgStore.CreateEnrollmentGrant(ctx, secondUser.ID, "", "grant-token-hash", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("PostgreSQL enrollment grant: %v", err)
	}
	if _, found, err := pgStore.EnrollDeviceWithGrant(ctx, "grant-token-hash", "new-router", "24.10"); err != nil || !found {
		t.Fatalf("PostgreSQL one-time enrollment: found=%v err=%v", found, err)
	}
	if _, found, err := pgStore.CreateRemoteSession(ctx, model.RemoteSession{
		DeviceID: device.DeviceID, RemotePort: 22001, LuCIPort: 22002,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}); err != nil || !found {
		t.Fatalf("PostgreSQL remote session: found=%v err=%v", found, err)
	}
	if _, found, err := pgStore.SyncDeviceAlerts(ctx, device.DeviceID, []model.Alert{{
		ID: "alert-1", Type: "cpu", Severity: "warning", Message: "CPU high", Details: []byte(`{"percent":90}`),
	}}); err != nil || !found {
		t.Fatalf("PostgreSQL alert sync: found=%v err=%v", found, err)
	}
	if _, err := pgStore.CreateAgentRollout(ctx, "stable", "1.2.3", 1, 1, []model.RolloutDevice{{
		DeviceID: device.DeviceID, FeedURL: "https://packages.example.test/feed", PackageManager: "opkg", PackageVersion: "1.2.3-1",
	}}); err != nil {
		t.Fatalf("PostgreSQL agent rollout: %v", err)
	}
	if err := pgStore.MigrateSensitiveData(ctx); err != nil {
		t.Fatalf("PostgreSQL sensitive data migration: %v", err)
	}
	_, err = Run(ctx, Options{
		SourceSQLite: path, TargetPostgres: targetURL, ExpectedSourceSHA256: hash,
		ConfirmCutover: true, DataEncryptionKey: dataKeyPath, CommandSigningKey: signingKeyPath,
		AllowInsecureLocal: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("expected populated-target guard, got %v", err)
	}
}

func TestFreshPostgresSchemaIntegration(t *testing.T) {
	targetURL := os.Getenv("RMM_TEST_POSTGRES_FRESH_URL")
	if targetURL == "" {
		t.Skip("set RMM_TEST_POSTGRES_FRESH_URL to an empty local PostgreSQL database")
	}
	u, err := url.Parse(targetURL)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Query().Get("sslmode") != "disable" {
		t.Fatal("integration test requires a local PostgreSQL URL with sslmode=disable")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", targetURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := EnsurePostgres(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePostgres(ctx, db); err != nil {
		t.Fatalf("idempotent PostgreSQL schema migration: %v", err)
	}
	st, err := store.OpenPostgres(ctx, targetURL, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	user, err := st.EnsureBootstrapUser(ctx, "admin", "test-password-hash")
	if err != nil || user.ID == "" {
		t.Fatalf("fresh PostgreSQL bootstrap: user=%#v err=%v", user, err)
	}
	device, err := st.EnrollDevice(ctx, "router", "24.10")
	if err != nil || device.DeviceID == "" {
		t.Fatalf("fresh PostgreSQL enrollment: device=%#v err=%v", device, err)
	}
	if _, err := st.SaveHeartbeat(ctx, device.DeviceID, []byte(`{"host":"router"}`), []byte(`{"cpu":8}`)); err != nil {
		t.Fatalf("fresh PostgreSQL heartbeat: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES (999, 'test-unknown-version')"); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePostgres(ctx, db); err == nil || !strings.Contains(err.Error(), "unknown migration version") {
		t.Fatalf("expected unknown PostgreSQL schema version rejection, got %v", err)
	}
}
