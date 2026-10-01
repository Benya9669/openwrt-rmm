package dbmigrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"rmm-openwrt/internal/fleetschema"
	"rmm-openwrt/server/internal/store"
	"testing"
)

func TestLegacyFleetMigrationIsReadOnlyAndPartialSchemaRejected(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-fixture.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.EnsureBootstrapUser(ctx, "legacy-admin", "test-hash"); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	fixture, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := parseSchema([]byte(fleetschema.SQL + "\n" + fleetschema.ManagementSQL))
	if err != nil {
		t.Fatal(err)
	}
	// Only this temporary fixture is edited to reproduce the published baseline.
	for i := len(tables) - 1; i >= 0; i-- {
		if _, err = fixture.ExecContext(ctx, `DROP TABLE "`+tables[i].name+`"`); err != nil {
			t.Fatal(err)
		}
	}
	if err = fixture.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true})
	if err != nil || report.Committed {
		t.Fatalf("legacy dry-run: %v", err)
	}
	for _, table := range report.Tables {
		if table.Table == "fleet_operations" {
			t.Fatal("legacy source acquired fleet tables")
		}
	}
	if after, err := fileSHA256(path); err != nil || after != hash {
		t.Fatal("legacy snapshot modified")
	}
	fixture, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.ExecContext(ctx, "CREATE TABLE fleet_assets (device_id TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err = fixture.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err = fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true}); err == nil {
		t.Fatal("partial fleet schema silently discarded")
	}
}

func TestManagementMigrationAcceptsFleetSnapshotWithoutChangingSource(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fleet-v2.db")
	st, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.EnsureBootstrapUser(ctx, "legacy-admin", "test-hash"); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	fixture, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := parseSchema([]byte(fleetschema.ManagementSQL))
	if err != nil {
		t.Fatal(err)
	}
	// Only this empty temporary fixture is edited to represent migration 2.
	for i := len(tables) - 1; i >= 0; i-- {
		if _, err = fixture.ExecContext(ctx, `DROP TABLE "`+tables[i].name+`"`); err != nil {
			t.Fatal(err)
		}
	}
	if err = fixture.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true})
	if err != nil || report.Committed {
		t.Fatalf("fleet v2 snapshot: %v", err)
	}
	for _, table := range report.Tables {
		if table.Table == "management_permissions" {
			t.Fatal("management tables appeared in old source")
		}
	}
	if after, err := fileSHA256(path); err != nil || after != hash {
		t.Fatal("fleet snapshot modified")
	}
	fixture, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.ExecContext(ctx, "CREATE TABLE management_permissions (user_id TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err = fixture.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err = fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Run(ctx, Options{SourceSQLite: path, ExpectedSourceSHA256: hash, DryRun: true}); err == nil {
		t.Fatal("partial management schema accepted")
	}
}
