package store

import (
	"strings"
	"testing"
)

func TestPostgresRebindSkipsQuotedSQL(t *testing.T) {
	query := `SELECT ?, '?', "?", $$?$$, $tag$?$tag$, /* ? */ ? -- ?
FROM x WHERE id = ?`
	want := `SELECT $1, '?', "?", $$?$$, $tag$?$tag$, /* ? */ $2 -- ?
FROM x WHERE id = $3`
	if got := rebindPostgres(query); got != want {
		t.Fatalf("rebindPostgres() = %q, want %q", got, want)
	}
}

func TestPostgresDialectConvertsSQLiteConstructs(t *testing.T) {
	query := `INSERT OR IGNORE INTO notification_deliveries (id, next_attempt_at) VALUES (?, ?) `
	if got := rewritePostgresSQL(query); got != `INSERT INTO notification_deliveries (id, next_attempt_at) VALUES ($1, $2) ON CONFLICT DO NOTHING` {
		t.Fatalf("insert ignore conversion = %q", got)
	}
	query = `SELECT 1 FROM users WHERE username = ? COLLATE NOCASE AND julianday(expires_at) > julianday(?)`
	got := rewritePostgresSQL(query)
	if !strings.Contains(got, "lower(username) = lower($1)") || !strings.Contains(got, "(expires_at::timestamptz) > ($2::timestamptz)") {
		t.Fatalf("SQLite predicates remain: %q", got)
	}
}

func TestPostgresArgsPreservesCallerAndConvertsFlags(t *testing.T) {
	args := []any{true, false, "value"}
	converted := postgresArgs(args)
	if converted[0] != int16(1) || converted[1] != int16(0) || converted[2] != "value" {
		t.Fatalf("unexpected converted arguments: %#v", converted)
	}
	if args[0] != true || args[1] != false {
		t.Fatalf("caller arguments changed: %#v", args)
	}
}
