package fleetschema

import _ "embed"

// SQL is shared by SQLite and PostgreSQL. TEXT feature timestamps and JSON
// documents preserve exact values when importing an existing SQLite snapshot.
//
//go:embed fleet.sql
var SQL string

// ManagementSQL is migration 3; keep migration 2 immutable.
//
//go:embed management.sql
var ManagementSQL string
