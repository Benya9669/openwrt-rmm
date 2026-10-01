package dbmigrate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"rmm-openwrt/internal/commandsig"
	"rmm-openwrt/internal/fieldcrypto"
	"rmm-openwrt/internal/fleetprofile"
	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func TestPostgresRecoveryIntegration(t *testing.T) {
	container := os.Getenv("RMM_TEST_POSTGRES_CONTAINER")
	if container == "" {
		t.Skip("set recovery URLs and isolated PostgreSQL container ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	open := func(env, name string) (*sql.DB, string) {
		t.Helper()
		dsn := os.Getenv(env)
		u, err := url.Parse(dsn)
		if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.Query().Get("sslmode") != "disable" || u.Path != "/"+name {
			t.Fatal("requires dedicated local recovery databases")
		}
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&count); err != nil || count != 0 {
			t.Fatal("recovery database must be empty; refusing to modify it")
		}
		return db, dsn
	}
	source, sourceDSN := open("RMM_TEST_POSTGRES_RECOVERY_SOURCE_URL", "rmm_recovery_source")
	restored, restoredDSN := open("RMM_TEST_POSTGRES_RECOVERY_TARGET_URL", "rmm_recovery_target")
	if err := EnsurePostgres(ctx, source); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	encryptionPath, signingPath := filepath.Join(dir, "data.key"), filepath.Join(dir, "signing.pem")
	cipher, err := fieldcrypto.LoadOrCreate(encryptionPath)
	if err != nil {
		t.Fatal(err)
	}
	private, err := commandsig.LoadOrCreatePrivateKey(signingPath)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.OpenPostgres(ctx, sourceDSN, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCommandSigningKey(private); err != nil {
		t.Fatal(err)
	}
	// Bootstrap through the actual HTTP handler so password hashing is realistic.
	config := httpapi.Config{OperatorUsername: "recovery-admin", OperatorPassword: "isolated-recovery-test-password"}
	httpapi.NewHandler(st, config)
	admin, _, found, err := st.GetUserByUsername(ctx, "recovery-admin")
	if err != nil || !found {
		t.Fatal("recovery administrator unavailable")
	}
	if _, err = st.SaveFleetProfile(ctx, admin.ID, store.FleetProfile{Title: "Recovery profile", Definition: fleetprofile.Profile{Config: "system", Options: []fleetprofile.Option{{Section: "core", Option: "hostname", Value: "recovery-router"}}}}); err != nil {
		t.Fatal(err)
	}
	device, err := st.EnrollDevice(ctx, "recovery-router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	_, command, found, err := st.RequestDeviceBackup(ctx, device.DeviceID)
	if err != nil || !found {
		t.Fatalf("request backup: %v", err)
	}
	if _, found, err := st.SaveDeviceBackup(ctx, device.DeviceID, command.ID, []byte("isolated archive fixture"), store.DeviceBackupMetadata{}); err != nil || !found {
		t.Fatalf("save archive: %v", err)
	}
	operator, err := st.CreateUser(ctx, "recovery-operator", "Recovery operator", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SavePermissionPolicy(ctx, admin.ID, operator.ID, store.PermissionPolicy{Permissions: []string{"view", "diagnostics"}, Groups: []string{"Recovery lab"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = st.SyncDeviceAlerts(ctx, device.DeviceID, []model.Alert{{ID: "recovery-incident-alert", Type: "offline", Message: "Synthetic recovery incident"}}); err != nil {
		t.Fatal(err)
	}
	want, err := VerifyRecovery(ctx, source, encryptionPath, signingPath)
	if err != nil {
		t.Fatal(err)
	}
	if want.EncryptedBackups != 1 || want.SignedCommands != 1 || want.EncryptedProfiles != 1 {
		t.Fatalf("incomplete recovery fixture: %+v", want)
	}
	// The server-version-matched client runs inside the disposable test service.
	dump := exec.CommandContext(ctx, "docker", "exec", container, "pg_dump", "-U", "rmm_test", "-d", "rmm_recovery_source", "--format=custom", "--no-owner", "--no-acl")
	data, err := dump.Output()
	if err != nil {
		t.Fatal("pg_dump failed")
	}
	restore := exec.CommandContext(ctx, "docker", "exec", "-i", container, "pg_restore", "-U", "rmm_test", "-d", "rmm_recovery_target", "--exit-on-error", "--single-transaction", "--no-owner", "--no-acl")
	restore.Stdin = bytes.NewReader(data)
	if err := restore.Run(); err != nil {
		t.Fatal("pg_restore failed")
	}
	got, err := VerifyRecovery(ctx, restored, encryptionPath, signingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatal("restored row counts or full-row hashes differ")
	}
	if image := os.Getenv("RMM_TEST_RECOVERY_IMAGE"); image != "" {
		dumpPath, reportPath := filepath.Join(dir, "backup.dump"), filepath.Join(dir, "expected.json")
		if err := os.WriteFile(dumpPath, data, 0600); err != nil {
			t.Fatal(err)
		}
		reportJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reportPath, reportJSON, 0600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		scriptArgs := []string{"../../../scripts/verify-postgres-backup.sh", "--dump", dumpPath, "--sha256", hex.EncodeToString(digest[:]), "--encryption-key", encryptionPath, "--signing-key", signingPath, "--image", image, "--postgres-image", os.Getenv("RMM_TEST_RECOVERY_POSTGRES_IMAGE"), "--expected", reportPath}
		check := exec.CommandContext(ctx, "bash", scriptArgs...)
		if output, err := check.CombinedOutput(); err != nil {
			t.Fatalf("isolated recovery script failed: %v: %s", err, output)
		}
		// Hash guard must fail before creating any restore resources.
		scriptArgs[4] = strings.Repeat("0", 64)
		check = exec.CommandContext(ctx, "bash", scriptArgs...)
		if output, err := check.CombinedOutput(); err == nil || !strings.Contains(string(output), "SHA-256 mismatch") {
			t.Fatal("script did not reject a mismatched dump hash")
		}
	}
	wrongPath := filepath.Join(dir, "wrong.key")
	if _, err := fieldcrypto.LoadOrCreate(wrongPath); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRecovery(ctx, restored, wrongPath, signingPath); err == nil {
		t.Fatal("wrong encryption key accepted")
	}
	wrongSigning := filepath.Join(dir, "wrong.pem")
	if _, err := commandsig.LoadOrCreatePrivateKey(wrongSigning); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRecovery(ctx, restored, encryptionPath, wrongSigning); err == nil {
		t.Fatal("wrong signing key accepted")
	}
	copyStore, err := store.OpenPostgres(ctx, restoredDSN, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer copyStore.Close()
	if err := copyStore.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	if err := copyStore.SetCommandSigningKey(private); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.NewHandler(copyStore, config))
	defer srv.Close()
	request := func(path, body, token string) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("restored %s returned %d", path, response.StatusCode)
		}
	}
	request("/api/auth/login", `{"username":"recovery-admin","password":"isolated-recovery-test-password"}`, "")
	request("/api/agent/heartbeat", `{"device_id":"`+device.DeviceID+`","inventory":{},"metrics":{}}`, device.DeviceToken)
	if _, err := restored.ExecContext(ctx, "UPDATE commands SET signature='corrupt' WHERE id=$1", command.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRecovery(ctx, restored, encryptionPath, signingPath); err == nil {
		t.Fatal("corrupt command accepted")
	}
	if _, err := restored.ExecContext(ctx, "UPDATE device_backups SET archive_ciphertext=$1", []byte("corrupt")); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRecovery(ctx, restored, encryptionPath, signingPath); err == nil {
		t.Fatal("corrupt encrypted archive accepted")
	}
}
