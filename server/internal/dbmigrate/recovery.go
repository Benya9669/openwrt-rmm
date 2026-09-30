package dbmigrate

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"rmm-openwrt/internal/commandsig"
	"rmm-openwrt/internal/fieldcrypto"
)

type RecoveryTable struct {
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}

type RecoveryReport struct {
	Tables           map[string]RecoveryTable `json:"tables"`
	SignedCommands   int                      `json:"signed_commands"`
	EncryptedBackups int                      `json:"encrypted_backups"`
}

// VerifyRecovery uses a consistent read-only snapshot. It never creates keys,
// migrates a database, starts workers, or includes row contents in its report.
func VerifyRecovery(ctx context.Context, db *sql.DB, encryptionPath, signingPath string) (RecoveryReport, error) {
	report := RecoveryReport{Tables: make(map[string]RecoveryTable)}
	keyText, err := os.ReadFile(encryptionPath)
	if err != nil {
		return report, errors.New("read existing encryption key failed")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(keyText)))
	if err != nil {
		return report, errors.New("invalid encryption key encoding")
	}
	cipher, err := fieldcrypto.New(raw)
	if err != nil {
		return report, errors.New("invalid encryption key")
	}
	private, err := commandsig.LoadPrivateKey(signingPath)
	if err != nil {
		return report, errors.New("read existing signing key failed")
	}
	public := private.Public().(ed25519.PublicKey)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return report, errors.New("begin recovery snapshot failed")
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO public; SET LOCAL timezone TO 'UTC'"); err != nil {
		return report, errors.New("configure recovery snapshot failed")
	}
	if err := checkPostgresVersion(ctx, tx); err != nil {
		return report, err
	}
	var migrationCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil || migrationCount != len(postgresMigrations) {
		return report, errors.New("migration set differs from this verifier")
	}
	for _, migration := range postgresMigrations {
		data, err := schemaFS.ReadFile(migration.file)
		if err != nil {
			return report, err
		}
		digest := sha256.Sum256(data)
		var recorded string
		if err := tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", migration.version).Scan(&recorded); err != nil || recorded != hex.EncodeToString(digest[:]) {
			return report, errors.New("migration checksum differs from this verifier")
		}
	}
	for name, want := range map[string]string{"data_encryption_key_id": cipher.KeyID(), "command_signing_key_id": commandsig.KeyID(public)} {
		var got string
		if err := tx.QueryRowContext(ctx, "SELECT value FROM security_metadata WHERE name=$1", name).Scan(&got); err != nil || got != want {
			return report, fmt.Errorf("%s does not match the supplied key", name)
		}
	}
	if err := validateEncryptedSample(ctx, tx, cipher); err != nil {
		return report, err
	}
	var missingArchives int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_backups WHERE status='ready' AND archive_ciphertext IS NULL").Scan(&missingArchives); err != nil || missingArchives != 0 {
		return report, errors.New("ready backup has no readable archive")
	}
	rows, err := tx.QueryContext(ctx, "SELECT device_id, command_id, archive_ciphertext, sha256, size_bytes FROM device_backups WHERE archive_ciphertext IS NOT NULL")
	if err != nil {
		return report, errors.New("read backup archives failed")
	}
	for rows.Next() {
		var device, command string
		var archive []byte
		var expectedSHA string
		var expectedSize int64
		if err := rows.Scan(&device, &command, &archive, &expectedSHA, &expectedSize); err != nil {
			rows.Close()
			return report, errors.New("read backup archive failed")
		}
		plaintext, err := cipher.DecryptBytesWithContext("device_backup\x00"+device+":"+command+"\x00archive", archive)
		if err != nil {
			rows.Close()
			return report, errors.New("backup archive authentication failed")
		}
		digest := sha256.Sum256(plaintext)
		if expectedSHA != hex.EncodeToString(digest[:]) || expectedSize != int64(len(plaintext)) {
			rows.Close()
			return report, errors.New("backup archive size or checksum differs")
		}
		report.EncryptedBackups++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, errors.New("read backup archives failed")
	}
	rows, err = tx.QueryContext(ctx, "SELECT id, device_id, type, args_json, created_at, expires_at, nonce, signature, signature_key_id FROM commands WHERE signature <> ''")
	if err != nil {
		return report, errors.New("read signed commands failed")
	}
	for rows.Next() {
		var envelope commandsig.Envelope
		var signature, keyID string
		if err := rows.Scan(&envelope.ID, &envelope.DeviceID, &envelope.Type, &envelope.Args, &envelope.CreatedAt, &envelope.ExpiresAt, &envelope.Nonce, &signature, &keyID); err != nil {
			rows.Close()
			return report, errors.New("read signed command failed")
		}
		if keyID != commandsig.KeyID(public) {
			rows.Close()
			return report, errors.New("command signing key ID differs")
		}
		if err := commandsig.Verify(public, envelope, signature); err != nil {
			rows.Close()
			return report, errors.New("command signature verification failed")
		}
		report.SignedCommands++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, errors.New("read signed commands failed")
	}
	// Hash PostgreSQL's canonical JSON row representation, ordered by that same
	// representation. This includes every column and preserves duplicate rows.
	rows, err = tx.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name")
	if err != nil {
		return report, errors.New("read recovery table list failed")
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return report, errors.New("read recovery table name failed")
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, errors.New("read recovery table list failed")
	}
	for _, name := range names {
		quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		var count int64
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoted).Scan(&count); err != nil {
			return report, errors.New("count recovery rows failed")
		}
		digest, err := rowDigest(ctx, tx, "SELECT to_jsonb(t)::text FROM "+quoted+" t ORDER BY to_jsonb(t)::text COLLATE \"C\"")
		if err != nil {
			return report, errors.New("hash recovery rows failed")
		}
		report.Tables[name] = RecoveryTable{Rows: count, SHA256: digest}
	}
	return report, nil
}
