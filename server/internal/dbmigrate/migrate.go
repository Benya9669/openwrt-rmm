package dbmigrate

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
	"rmm-openwrt/internal/commandsig"
	"rmm-openwrt/internal/fieldcrypto"
	"rmm-openwrt/internal/fleetschema"
)

//go:embed postgres.sql
var schemaFS embed.FS

type Options struct {
	SourceSQLite         string
	TargetPostgres       string
	ExpectedSourceSHA256 string
	ConfirmCutover       bool
	DryRun               bool
	DataEncryptionKey    string
	CommandSigningKey    string
	AllowInsecureLocal   bool
}

type TableCount struct {
	Table  string `json:"table"`
	Source int64  `json:"source"`
	Target int64  `json:"target,omitempty"`
}

type Report struct {
	SourceSHA256  string       `json:"source_sha256"`
	SchemaSHA256  string       `json:"schema_sha256"`
	DryRun        bool         `json:"dry_run"`
	Committed     bool         `json:"committed"`
	KeysValidated bool         `json:"keys_validated"`
	Tables        []TableCount `json:"tables"`
	Duration      string       `json:"duration"`
}

type columnSpec struct {
	name     string
	dataType string
	notNull  bool
}

type tableSpec struct {
	name    string
	columns []columnSpec
}

var createTablePattern = regexp.MustCompile(`(?ms)^CREATE TABLE ([a-z_]+) \(\r?\n(.*?)^\);`)
var identifierPattern = regexp.MustCompile(`^[a-z_][a-z_0-9]*$`)

// Run validates a stopped SQLite snapshot and imports it into an empty
// PostgreSQL database. PostgreSQL schema and data are committed together.
func Run(ctx context.Context, options Options) (Report, error) {
	started := time.Now()
	report := Report{DryRun: options.DryRun, Tables: []TableCount{}}
	if options.SourceSQLite == "" || options.ExpectedSourceSHA256 == "" {
		return report, errors.New("source SQLite path and expected SHA-256 are required")
	}
	if len(options.ExpectedSourceSHA256) != 64 {
		return report, errors.New("expected source SHA-256 must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(options.ExpectedSourceSHA256); err != nil {
		return report, errors.New("expected source SHA-256 is invalid")
	}
	if !options.DryRun {
		if !options.ConfirmCutover {
			return report, errors.New("--confirm-cutover is required for an import")
		}
		if options.DataEncryptionKey == "" || options.CommandSigningKey == "" {
			return report, errors.New("data-encryption-key-file and command-signing-key-file are required for an import")
		}
		if err := validatePostgresURL(options.TargetPostgres, options.AllowInsecureLocal); err != nil {
			return report, err
		}
	}

	schema, err := schemaFS.ReadFile("postgres.sql")
	if err != nil {
		return report, err
	}
	schemaHash := sha256.Sum256(schema)
	report.SchemaSHA256 = hex.EncodeToString(schemaHash[:])
	schema = append(schema, []byte("\n"+fleetschema.SQL+"\n"+fleetschema.ManagementSQL)...)
	tables, err := parseSchema(schema)
	if err != nil {
		return report, err
	}

	sourcePath, err := filepath.Abs(options.SourceSQLite)
	if err != nil {
		return report, err
	}
	if err := checkSourceFile(sourcePath); err != nil {
		return report, err
	}
	report.SourceSHA256, err = fileSHA256(sourcePath)
	if err != nil {
		return report, err
	}
	if !strings.EqualFold(report.SourceSHA256, options.ExpectedSourceSHA256) {
		return report, errors.New("source SQLite SHA-256 does not match the expected value")
	}

	// The URI requests read-only access. The source is never passed to the
	// application's OpenSQLite function, which runs write migrations.
	source, err := sql.Open("sqlite", readOnlySQLiteURI(sourcePath))
	if err != nil {
		return report, fmt.Errorf("open source SQLite: %w", err)
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	sourceTx, err := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, fmt.Errorf("begin source read transaction: %w", err)
	}
	defer sourceTx.Rollback()
	// Baseline snapshots predate fleet tables. A partial fleet schema remains
	// invalid, but an older snapshot can still be imported without alteration.
	var fleetTables int
	if err := sourceTx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name GLOB 'fleet_*'").Scan(&fleetTables); err != nil {
		return report, err
	}
	if fleetTables == 0 {
		baseline, err := schemaFS.ReadFile("postgres.sql")
		if err != nil {
			return report, err
		}
		tables, err = parseSchema(baseline)
		if err != nil {
			return report, err
		}
	}
	var managementTables int
	if err := sourceTx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name GLOB 'management_*'").Scan(&managementTables); err != nil {
		return report, err
	}
	if managementTables == 0 {
		filtered := tables[:0]
		for _, table := range tables {
			if !strings.HasPrefix(table.name, "management_") {
				filtered = append(filtered, table)
			}
		}
		tables = filtered
	}
	if err := validateSource(ctx, sourceTx, tables); err != nil {
		return report, err
	}
	if options.DataEncryptionKey != "" || options.CommandSigningKey != "" {
		if options.DataEncryptionKey == "" || options.CommandSigningKey == "" {
			return report, errors.New("both key files are required for key validation")
		}
		if err := validateKeys(ctx, sourceTx, options); err != nil {
			return report, err
		}
		report.KeysValidated = true
	}

	var target *sql.DB
	var targetTx *sql.Tx
	if !options.DryRun {
		target, err = sql.Open("pgx", options.TargetPostgres)
		if err != nil {
			return report, errors.New("open PostgreSQL connection failed")
		}
		defer target.Close()
		target.SetMaxOpenConns(1)
		if err := target.PingContext(ctx); err != nil {
			return report, errors.New("connect to PostgreSQL failed; check endpoint, TLS, and credentials")
		}
		targetTx, err = target.BeginTx(ctx, nil)
		if err != nil {
			return report, errors.New("begin PostgreSQL migration transaction failed")
		}
		defer targetTx.Rollback()
		if err := checkPostgresVersion(ctx, targetTx); err != nil {
			return report, err
		}
		if err := prepareTarget(ctx, targetTx, schema); err != nil {
			return report, err
		}
	}

	for _, table := range tables {
		if table.name == "schema_migrations" {
			continue
		}
		count, err := copyTable(ctx, sourceTx, targetTx, table)
		if err != nil {
			return report, fmt.Errorf("table %s: %w", table.name, err)
		}
		item := TableCount{Table: table.name, Source: count}
		if targetTx != nil {
			if err := targetTx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdentifier(table.name)).Scan(&item.Target); err != nil {
				return report, fmt.Errorf("count target table %s failed", table.name)
			}
			if item.Target != count {
				return report, fmt.Errorf("table %s count differs between source and target", table.name)
			}
		}
		report.Tables = append(report.Tables, item)
	}
	if err := checkSourceFile(sourcePath); err != nil {
		return report, err
	}
	afterHash, err := fileSHA256(sourcePath)
	if err != nil {
		return report, err
	}
	if afterHash != report.SourceSHA256 {
		return report, errors.New("source SQLite changed during migration")
	}
	if targetTx != nil {
		if err := validateTarget(ctx, sourceTx, targetTx, report); err != nil {
			return report, err
		}
		if err := targetTx.Commit(); err != nil {
			return report, errors.New("commit PostgreSQL migration failed")
		}
		report.Committed = true
	}
	report.Duration = time.Since(started).Round(time.Millisecond).String()
	return report, nil
}

func validatePostgresURL(raw string, allowInsecureLocal bool) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || strings.Trim(u.Path, "/") == "" {
		return errors.New("target PostgreSQL URL must contain a host and database name")
	}
	if u.Query().Get("sslmode") != "verify-full" {
		if allowInsecureLocal && u.Query().Get("sslmode") == "disable" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
			return nil
		}
		return errors.New("target PostgreSQL URL must use sslmode=verify-full")
	}
	return nil
}

// ValidatePostgresURL rejects ambiguous or insecure production configuration.
func ValidatePostgresURL(raw string, allowInsecureLocal bool) error {
	return validatePostgresURL(raw, allowInsecureLocal)
}

// AutoImport imports an existing SQLite file once, when the selected PostgreSQL
// database is empty. A populated target is left untouched on later starts.
func AutoImport(ctx context.Context, options Options) (bool, error) {
	if err := validatePostgresURL(options.TargetPostgres, options.AllowInsecureLocal); err != nil {
		return false, err
	}
	if options.SourceSQLite == "" {
		return false, nil
	}
	info, err := os.Stat(options.SourceSQLite)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return false, errors.New("source SQLite path is not a regular file")
	}
	target, err := sql.Open("pgx", options.TargetPostgres)
	if err != nil {
		return false, errors.New("open PostgreSQL database failed")
	}
	defer target.Close()
	if err := target.PingContext(ctx); err != nil {
		return false, errors.New("connect to PostgreSQL database failed")
	}
	var existing int
	if err := target.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'").Scan(&existing); err != nil {
		return false, errors.New("inspect PostgreSQL schema failed")
	}
	if existing != 0 {
		return false, nil
	}
	sourcePath, err := filepath.Abs(options.SourceSQLite)
	if err != nil {
		return false, err
	}
	wal, err := os.Stat(sourcePath + "-wal")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect source SQLite WAL: %w", err)
	}
	var hash string
	if err == nil && wal.Size() > 0 {
		sourcePath, hash, err = snapshotSQLiteWithWAL(ctx, sourcePath)
	} else {
		err = checkSourceFile(sourcePath)
		if err == nil {
			hash, err = fileSHA256(sourcePath)
		}
		if err == nil {
			err = preserveSQLiteCopy(sourcePath, hash)
		}
	}
	if err != nil {
		return false, err
	}
	options.SourceSQLite = sourcePath
	options.ExpectedSourceSHA256 = hash
	options.ConfirmCutover = true
	options.DryRun = false
	_, err = Run(ctx, options)
	if err != nil {
		return false, err
	}
	return true, nil
}

func readOnlySQLiteURI(sourcePath string) string {
	uriPath := filepath.ToSlash(sourcePath)
	if filepath.VolumeName(sourcePath) != "" {
		uriPath = "/" + uriPath
	}
	return (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}).String()
}

// snapshotSQLiteWithWAL includes committed WAL pages in a standalone recovery
// copy. The original database and WAL are never checkpointed or rewritten.
func snapshotSQLiteWithWAL(ctx context.Context, sourcePath string) (string, string, error) {
	beforeDB, err := fileSHA256(sourcePath)
	if err != nil {
		return "", "", err
	}
	beforeWAL, err := fileSHA256(sourcePath + "-wal")
	if err != nil {
		return "", "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(sourcePath), ".rmm-postgres-snapshot-*.db")
	if err != nil {
		return "", "", fmt.Errorf("create SQLite snapshot path: %w", err)
	}
	tempPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return "", "", err
	}
	if err := os.Remove(tempPath); err != nil {
		return "", "", err
	}
	defer os.Remove(tempPath)

	source, err := sql.Open("sqlite", readOnlySQLiteURI(sourcePath))
	if err != nil {
		return "", "", fmt.Errorf("open SQLite for snapshot: %w", err)
	}
	source.SetMaxOpenConns(1)
	// SQLite's VACUUM INTO reads one consistent transaction and writes a
	// self-contained database, including pages that are still in the WAL.
	query := "VACUUM main INTO '" + strings.ReplaceAll(tempPath, "'", "''") + "'"
	_, snapshotErr := source.ExecContext(ctx, query)
	closeErr := source.Close()
	if snapshotErr != nil {
		return "", "", fmt.Errorf("create consistent SQLite snapshot: %w", snapshotErr)
	}
	if closeErr != nil {
		return "", "", fmt.Errorf("close SQLite after snapshot: %w", closeErr)
	}
	snapshotFile, err := os.OpenFile(tempPath, os.O_RDWR, 0)
	if err != nil {
		return "", "", fmt.Errorf("open SQLite snapshot for sync: %w", err)
	}
	syncErr := snapshotFile.Sync()
	closeErr = snapshotFile.Close()
	if syncErr != nil || closeErr != nil {
		return "", "", errors.New("sync SQLite snapshot failed")
	}
	afterDB, dbErr := fileSHA256(sourcePath)
	afterWAL, walErr := fileSHA256(sourcePath + "-wal")
	if dbErr != nil || walErr != nil || afterDB != beforeDB || afterWAL != beforeWAL {
		return "", "", errors.New("source SQLite or WAL changed during snapshot; stop the SQLite server")
	}
	hash, err := fileSHA256(tempPath)
	if err != nil {
		return "", "", err
	}
	backupPath := sourcePath + ".pre-postgres-" + hash[:12] + ".db"
	if _, err := os.Stat(backupPath); err == nil {
		existingHash, hashErr := fileSHA256(backupPath)
		if hashErr != nil || existingHash != hash {
			return "", "", errors.New("existing pre-PostgreSQL SQLite copy has a different hash")
		}
		return backupPath, hash, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", errors.New("inspect pre-PostgreSQL SQLite copy failed")
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return "", "", fmt.Errorf("protect SQLite snapshot: %w", err)
	}
	if err := os.Rename(tempPath, backupPath); err != nil {
		return "", "", fmt.Errorf("preserve SQLite snapshot: %w", err)
	}
	return backupPath, hash, nil
}

func preserveSQLiteCopy(sourcePath, expectedHash string) error {
	backupPath := sourcePath + ".pre-postgres-" + expectedHash[:12] + ".db"
	if _, err := os.Stat(backupPath); err == nil {
		existingHash, hashErr := fileSHA256(backupPath)
		if hashErr != nil || existingHash != expectedHash {
			return errors.New("existing pre-PostgreSQL SQLite copy has a different hash")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("inspect pre-PostgreSQL SQLite copy failed")
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("create pre-PostgreSQL SQLite copy failed")
	}
	completed := false
	defer func() {
		backup.Close()
		if !completed {
			os.Remove(backupPath)
		}
	}()
	if _, err := io.Copy(backup, source); err != nil {
		return errors.New("copy SQLite before PostgreSQL import failed")
	}
	if err := backup.Sync(); err != nil {
		return errors.New("sync pre-PostgreSQL SQLite copy failed")
	}
	if err := backup.Close(); err != nil {
		return errors.New("close pre-PostgreSQL SQLite copy failed")
	}
	backupHash, err := fileSHA256(backupPath)
	if err != nil || backupHash != expectedHash {
		return errors.New("pre-PostgreSQL SQLite copy hash does not match the source")
	}
	completed = true
	return nil
}

func checkSourceFile(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("source SQLite file must exist and be a regular file")
	}
	if wal, err := os.Stat(path + "-wal"); err == nil && wal.Size() > 0 {
		return errors.New("source SQLite has a non-empty WAL; stop the server and use a consistent snapshot")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect source SQLite WAL: %w", err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func parseSchema(schema []byte) ([]tableSpec, error) {
	matches := createTablePattern.FindAllSubmatch(schema, -1)
	if len(matches) == 0 {
		return nil, errors.New("embedded PostgreSQL schema contains no tables")
	}
	var tables []tableSpec
	for _, match := range matches {
		table := tableSpec{name: string(match[1])}
		for _, line := range strings.Split(string(match[2]), "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, ","))
			if line == "" || strings.HasPrefix(line, "PRIMARY KEY ") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 2 || !identifierPattern.MatchString(parts[0]) {
				return nil, fmt.Errorf("invalid column in embedded schema table %s", table.name)
			}
			dataType := parts[1]
			if dataType == "TIMESTAMPTZ" || dataType == "SMALLINT" || dataType == "JSONB" || dataType == "BYTEA" || dataType == "TEXT" || dataType == "INTEGER" || dataType == "BIGINT" {
				table.columns = append(table.columns, columnSpec{name: parts[0], dataType: dataType, notNull: strings.Contains(line, "NOT NULL") || strings.Contains(line, "PRIMARY KEY")})
			} else {
				return nil, fmt.Errorf("unsupported embedded schema type %s", dataType)
			}
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func quoteIdentifier(name string) string { return `"` + name + `"` }

func validateSource(ctx context.Context, source *sql.Tx, tables []tableSpec) error {
	var result string
	if err := source.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil || result != "ok" {
		return errors.New("source SQLite quick_check failed")
	}
	rows, err := source.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("source SQLite foreign_key_check: %w", err)
	}
	hasViolation := rows.Next()
	rowErr := rows.Err()
	closeErr := rows.Close()
	if hasViolation || rowErr != nil || closeErr != nil {
		return errors.New("source SQLite foreign_key_check failed")
	}
	want := make([]string, 0, len(tables))
	for _, table := range tables {
		if table.name != "schema_migrations" {
			want = append(want, table.name)
		}
	}
	slices.Sort(want)
	rows, err = source.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return err
	}
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		got = append(got, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !slices.Equal(got, want) {
		return errors.New("source SQLite table set differs from the supported migration schema")
	}
	for _, statement := range []string{
		"SELECT lower(username) FROM users GROUP BY lower(username) HAVING COUNT(*) > 1 LIMIT 1",
		"SELECT lower(email) FROM users WHERE email <> '' GROUP BY lower(email) HAVING COUNT(*) > 1 LIMIT 1",
	} {
		var duplicate string
		err := source.QueryRowContext(ctx, statement).Scan(&duplicate)
		if err == nil {
			return errors.New("source SQLite has case-insensitive duplicate user credentials")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

func validateKeys(ctx context.Context, source *sql.Tx, options Options) error {
	for _, path := range []string{options.DataEncryptionKey, options.CommandSigningKey} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("key file must exist and be a regular file")
		}
	}
	dataKeyBytes, err := os.ReadFile(options.DataEncryptionKey)
	if err != nil {
		return errors.New("read data encryption key failed")
	}
	rawDataKey, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(dataKeyBytes)))
	if err != nil {
		return errors.New("decode data encryption key failed")
	}
	cipher, err := fieldcrypto.New(rawDataKey)
	if err != nil {
		return errors.New("data encryption key is invalid")
	}
	privateKey, err := commandsig.LoadPrivateKey(options.CommandSigningKey)
	if err != nil {
		return errors.New("command signing key is invalid")
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return errors.New("command signing key is invalid")
	}
	for name, expected := range map[string]string{
		"data_encryption_key_id": cipher.KeyID(),
		"command_signing_key_id": commandsig.KeyID(publicKey),
	} {
		var pinned string
		if err := source.QueryRowContext(ctx, "SELECT value FROM security_metadata WHERE name = ?", name).Scan(&pinned); err != nil || pinned != expected {
			return fmt.Errorf("%s does not match the source database", name)
		}
	}
	if err := validateEncryptedSample(ctx, source, cipher); err != nil {
		return err
	}
	return nil
}

func validateEncryptedSample(ctx context.Context, source *sql.Tx, cipher *fieldcrypto.Cipher) error {
	type sample struct {
		query      string
		recordType string
		field      string
	}
	for _, item := range []sample{
		{"SELECT user_id, telegram_chat_id FROM notification_settings WHERE telegram_chat_id LIKE 'enc:v1:%' LIMIT 1", "notification_settings", "telegram_chat_id"},
		{"SELECT user_id, webhook_url FROM notification_settings WHERE webhook_url LIKE 'enc:v1:%' LIMIT 1", "notification_settings", "webhook_url"},
		{"SELECT user_id, webhook_secret FROM notification_settings WHERE webhook_secret LIKE 'enc:v1:%' LIMIT 1", "notification_settings", "webhook_secret"},
		{"SELECT id, title FROM notification_deliveries WHERE title LIKE 'enc:v1:%' LIMIT 1", "notification_delivery", "title"},
		{"SELECT id, body FROM notification_deliveries WHERE body LIKE 'enc:v1:%' LIMIT 1", "notification_delivery", "body"},
		{"SELECT id, destination FROM notification_deliveries WHERE destination LIKE 'enc:v1:%' LIMIT 1", "notification_delivery", "destination"},
		{"SELECT user_id || ':' || channel, destination FROM contact_verifications WHERE destination LIKE 'enc:v1:%' LIMIT 1", "contact_verification", "destination"},
	} {
		var id, value string
		err := source.QueryRowContext(ctx, item.query).Scan(&id, &value)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return errors.New("read encrypted field sample failed")
		}
		context := item.recordType + "\x00" + id + "\x00" + item.field
		if _, err := cipher.DecryptWithContext(context, value); err != nil {
			return errors.New("encrypted field sample cannot be decrypted with the provided key")
		}
	}
	var deviceID, commandID string
	var archive []byte
	err := source.QueryRowContext(ctx, "SELECT device_id, command_id, archive_ciphertext FROM device_backups WHERE archive_ciphertext IS NOT NULL LIMIT 1").Scan(&deviceID, &commandID, &archive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return errors.New("read encrypted backup sample failed")
	}
	context := "device_backup\x00" + deviceID + ":" + commandID + "\x00archive"
	if _, err := cipher.DecryptBytesWithContext(context, archive); err != nil {
		return errors.New("encrypted backup sample cannot be decrypted with the provided key")
	}
	return nil
}

func prepareTarget(ctx context.Context, tx *sql.Tx, schema []byte) error {
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO public"); err != nil {
		return errors.New("set PostgreSQL schema failed")
	}
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(7766442211)"); err != nil {
		return errors.New("acquire PostgreSQL migration lock failed")
	}
	var existing int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'").Scan(&existing); err != nil {
		return errors.New("inspect PostgreSQL schema failed")
	}
	if existing != 0 {
		return errors.New("target PostgreSQL public schema is not empty")
	}
	for _, statement := range strings.Split(string(schema), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return errors.New("create PostgreSQL baseline schema failed")
		}
	}
	return nil
}

// Add new, ordered SQL files here for future PostgreSQL schema upgrades.
var postgresMigrations = []struct {
	version int
	file    string
}{{1, "postgres.sql"}, {2, "fleet.sql"}, {3, "management.sql"}}

func readMigration(file string) ([]byte, error) {
	if file == "management.sql" {
		return []byte(fleetschema.ManagementSQL), nil
	}
	if file == "fleet.sql" {
		return []byte(fleetschema.SQL), nil
	}
	return schemaFS.ReadFile(file)
}

func checkPostgresVersion(ctx context.Context, tx *sql.Tx) error {
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::integer").Scan(&version); err != nil {
		return errors.New("read PostgreSQL server version failed")
	}
	major := version / 10000
	if major < 16 || major > 18 {
		return fmt.Errorf("unsupported PostgreSQL major version %d; supported versions are 16 through 18", major)
	}
	return nil
}

// EnsurePostgres applies pending versioned migrations in one transaction and
// verifies the checksum of every previously applied version.
func EnsurePostgres(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("begin PostgreSQL schema migration failed")
	}
	defer tx.Rollback()
	if err := checkPostgresVersion(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO public"); err != nil {
		return errors.New("set PostgreSQL schema failed")
	}
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(7766442211)"); err != nil {
		return errors.New("acquire PostgreSQL migration lock failed")
	}
	var existing int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'").Scan(&existing); err != nil {
		return errors.New("inspect PostgreSQL schema failed")
	}
	applied := make(map[int]string)
	if existing != 0 {
		rows, err := tx.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
		if err != nil {
			return errors.New("PostgreSQL schema has no migration marker")
		}
		for rows.Next() {
			var version int
			var checksum string
			if err := rows.Scan(&version, &checksum); err != nil {
				rows.Close()
				return errors.New("read PostgreSQL schema migrations failed")
			}
			applied[version] = checksum
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return errors.New("read PostgreSQL schema migrations failed")
		}
		rows.Close()
		if len(applied) == 0 || applied[1] == "" {
			return errors.New("PostgreSQL schema has no baseline migration marker")
		}
	}
	for _, migration := range postgresMigrations {
		sqlText, err := readMigration(migration.file)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(sqlText)
		checksum := hex.EncodeToString(digest[:])
		if recorded, ok := applied[migration.version]; ok {
			if recorded != checksum {
				return fmt.Errorf("PostgreSQL schema migration %d checksum differs from this server", migration.version)
			}
			delete(applied, migration.version)
			continue
		}
		for _, statement := range strings.Split(string(sqlText), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply PostgreSQL schema migration %d failed", migration.version)
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)", migration.version, checksum); err != nil {
			return fmt.Errorf("record PostgreSQL schema migration %d failed", migration.version)
		}
	}
	if len(applied) != 0 {
		return errors.New("PostgreSQL schema contains an unknown migration version")
	}
	if err := tx.Commit(); err != nil {
		return errors.New("commit PostgreSQL schema migration failed")
	}
	return nil
}

func copyTable(ctx context.Context, source, target *sql.Tx, table tableSpec) (int64, error) {
	rows, err := source.QueryContext(ctx, "SELECT * FROM "+quoteIdentifier(table.name))
	if err != nil {
		return 0, fmt.Errorf("read source rows: %w", err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	if len(names) != len(table.columns) {
		return 0, errors.New("source columns differ from PostgreSQL baseline")
	}
	specByName := make(map[string]columnSpec, len(table.columns))
	for _, column := range table.columns {
		specByName[column.name] = column
	}
	for _, name := range names {
		if _, ok := specByName[name]; !ok {
			return 0, errors.New("source columns differ from PostgreSQL baseline")
		}
	}
	var count int64
	var batch [][]any
	var batchBytes int
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return 0, err
		}
		converted := make([]any, len(values))
		for i, value := range values {
			converted[i], err = convertValue(value, specByName[names[i]])
			if err != nil {
				return 0, fmt.Errorf("column %s: %w", names[i], err)
			}
			if b, ok := value.([]byte); ok {
				batchBytes += len(b)
			} else if s, ok := value.(string); ok {
				batchBytes += len(s)
			}
		}
		count++
		if target != nil {
			batch = append(batch, converted)
			if len(batch) >= 100 || batchBytes >= 4*1024*1024 {
				if err := insertBatch(ctx, target, table.name, names, batch); err != nil {
					return 0, err
				}
				batch = nil
				batchBytes = 0
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(batch) > 0 {
		if err := insertBatch(ctx, target, table.name, names, batch); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func convertValue(value any, column columnSpec) (any, error) {
	if value == nil {
		if column.notNull {
			return nil, errors.New("required value is NULL")
		}
		return nil, nil
	}
	switch column.dataType {
	case "TIMESTAMPTZ":
		text := asText(value)
		if text == "" && !column.notNull {
			return nil, nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return nil, errors.New("invalid RFC3339 timestamp")
		}
		return parsed.UTC(), nil
	case "JSONB":
		text := asText(value)
		if !json.Valid([]byte(text)) {
			return nil, errors.New("invalid JSON")
		}
		return text, nil
	case "SMALLINT":
		switch value {
		case int64(0):
			return int64(0), nil
		case int64(1):
			return int64(1), nil
		default:
			return nil, errors.New("boolean must be 0 or 1")
		}
	case "BYTEA":
		bytes, ok := value.([]byte)
		if !ok {
			return nil, errors.New("invalid BLOB")
		}
		return bytes, nil
	case "TEXT":
		if _, ok := value.(string); !ok {
			return nil, errors.New("invalid text")
		}
		if column.name == "next_attempt_at" && value != "" {
			if _, err := time.Parse(time.RFC3339Nano, value.(string)); err != nil {
				return nil, errors.New("invalid next attempt timestamp")
			}
		}
		return value, nil
	case "INTEGER", "BIGINT":
		if _, ok := value.(int64); !ok {
			return nil, errors.New("invalid integer")
		}
		return value, nil
	default:
		return nil, errors.New("unsupported column type")
	}
}

func asText(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if b, ok := value.([]byte); ok {
		return string(b)
	}
	return ""
}

func insertBatch(ctx context.Context, tx *sql.Tx, table string, columns []string, batch [][]any) error {
	var query strings.Builder
	query.WriteString("INSERT INTO " + quoteIdentifier(table) + " (")
	for i, column := range columns {
		if i > 0 {
			query.WriteByte(',')
		}
		query.WriteString(quoteIdentifier(column))
	}
	query.WriteString(") VALUES ")
	args := make([]any, 0, len(columns)*len(batch))
	for rowIndex, row := range batch {
		if rowIndex > 0 {
			query.WriteByte(',')
		}
		query.WriteByte('(')
		for colIndex, value := range row {
			if colIndex > 0 {
				query.WriteByte(',')
			}
			query.WriteString(fmt.Sprintf("$%d", len(args)+1))
			args = append(args, value)
		}
		query.WriteByte(')')
	}
	if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
		return errors.New("insert PostgreSQL rows failed")
	}
	return nil
}

func validateTarget(ctx context.Context, source, target *sql.Tx, report Report) error {
	// Compare security metadata exactly, including pinned signing and data-key IDs.
	sourceHash, err := metadataHash(ctx, source)
	if err != nil {
		return err
	}
	targetHash, err := metadataHash(ctx, target)
	if err != nil || sourceHash != targetHash {
		return errors.New("security metadata differs after import")
	}
	for _, spec := range []struct {
		table  string
		fields string
		order  string
	}{
		{"devices", "id, token, token_hash, next_token_hash, next_token_ciphertext, tunnel_public_key, tunnel_key_fingerprint", "id"},
		{"commands", "id, nonce, signature_key_id, signature", "id"},
		{"device_backups", "id, archive_ciphertext, sha256", "id"},
		{"notification_settings", "user_id, telegram_chat_id, webhook_url, webhook_secret", "user_id"},
		{"notification_deliveries", "id, title, body, destination", "id"},
		{"contact_verifications", "user_id, channel, destination", "user_id, channel"},
	} {
		query := "SELECT " + spec.fields + " FROM " + spec.table + " ORDER BY " + spec.order
		sourceDigest, err := rowDigest(ctx, source, query)
		if err != nil {
			return fmt.Errorf("hash source %s: %w", spec.table, err)
		}
		targetDigest, err := rowDigest(ctx, target, query)
		if err != nil || sourceDigest != targetDigest {
			return fmt.Errorf("critical values differ after import in %s", spec.table)
		}
	}
	if _, err := target.ExecContext(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES (1, $1)", report.SchemaSHA256); err != nil {
		return errors.New("record PostgreSQL schema version failed")
	}
	for _, migration := range postgresMigrations[1:] {
		data, err := readMigration(migration.file)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		if _, err := target.ExecContext(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)", migration.version, hex.EncodeToString(digest[:])); err != nil {
			return errors.New("record PostgreSQL fleet schema version failed")
		}
	}
	if _, err := target.ExecContext(ctx, "CREATE TABLE database_imports (source_sha256 TEXT PRIMARY KEY, imported_at TIMESTAMPTZ NOT NULL DEFAULT now(), table_counts JSONB NOT NULL)"); err != nil {
		return errors.New("create PostgreSQL import record failed")
	}
	counts, err := json.Marshal(report.Tables)
	if err != nil {
		return err
	}
	if _, err := target.ExecContext(ctx, "INSERT INTO database_imports (source_sha256, table_counts) VALUES ($1, $2)", report.SourceSHA256, string(counts)); err != nil {
		return errors.New("record PostgreSQL import metadata failed")
	}
	if _, err := target.ExecContext(ctx, "SAVEPOINT rmm_migration_smoke_test"); err != nil {
		return errors.New("PostgreSQL transaction smoke test failed")
	}
	if _, err := target.ExecContext(ctx, "ROLLBACK TO SAVEPOINT rmm_migration_smoke_test"); err != nil {
		return errors.New("PostgreSQL transaction rollback smoke test failed")
	}
	return nil
}

func rowDigest(ctx context.Context, tx *sql.Tx, query string) (string, error) {
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	var length [8]byte
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return "", err
		}
		for _, value := range values {
			if value == nil {
				binary.LittleEndian.PutUint64(length[:], ^uint64(0))
				h.Write(length[:])
				continue
			}
			var data []byte
			switch typed := value.(type) {
			case string:
				data = []byte(typed)
			case []byte:
				data = typed
			default:
				return "", errors.New("critical column has an unexpected type")
			}
			binary.LittleEndian.PutUint64(length[:], uint64(len(data)))
			h.Write(length[:])
			h.Write(data)
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func metadataHash(ctx context.Context, tx *sql.Tx) (string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name, value FROM security_metadata ORDER BY name")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s%d:%s", len(name), name, len(value), value)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
