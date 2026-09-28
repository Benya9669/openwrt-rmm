# PostgreSQL backend and automatic migration

For an existing production deployment, follow the
[step-by-step production cutover guide](postgres-production-migration.md).

The server supports SQLite (default) and PostgreSQL 16-18. The Compose override
starts PostgreSQL 18 on a private network and selects it for RMM. On startup, an empty PostgreSQL
database receives the current schema. If `/data/rmm.db` exists, the server first
creates a standalone recovery copy at `/data/rmm.db.pre-postgres-<sha256-prefix>.db`,
then imports all data
in one PostgreSQL transaction. The source SQLite file, signing key and encryption
key stay in the existing `rmm-data` volume. Committed data still in SQLite's WAL
is included in the recovery copy; the original database and WAL are not changed.
Later starts reuse the populated
PostgreSQL database and do not import again. If validation or import fails, the
server does not start and the PostgreSQL transaction rolls back.

## Configure and run

1. Stop the SQLite RMM server before switching backends. Back up the complete
   `rmm-data` volume, including `rmm.db`, any `rmm.db-wal` and `rmm.db-shm`, and
   both key files. Do not copy a live SQLite file. The automatic importer makes
   a consistent snapshot if the stopped database has a non-empty WAL.
2. Set strong, different `RMM_POSTGRES_PASSWORD` and
   `RMM_POSTGRES_ADMIN_PASSWORD` values in the local deployment environment or
   untracked `.env`. The application role can create tables in the RMM database
   but is not a PostgreSQL superuser. Do not commit credentials. Ensure there is space for the SQLite
   recovery copy, PostgreSQL data, WAL, and backups.
3. From the repository root, run:

   ```sh
   docker compose -f compose.yaml stop rmm-server
   docker compose -f compose.yaml -f compose.postgres.yaml pull rmm-server tunnel-ssh postgres postgres-tls-init
   docker compose -f compose.yaml -f compose.postgres.yaml up -d
   ```

   The override uses pinned release images for the server and one-shot certificate initializer, starts
   PostgreSQL 18 without a published database port, and keeps the existing
   `rmm-data` volume. It creates separate volumes for PostgreSQL data, the CA
   signing key, the server certificate, and the client CA certificate. Only the
   CA certificate is mounted into RMM. PostgreSQL rejects non-TLS network
   connections; RMM uses `sslmode=verify-full` for the internal `postgres` name.
   For a fresh installation without SQLite, the server creates the schema. For a
   local source build, add both `-f compose.dev.yaml` and
   `-f compose.postgres.dev.yaml` after the PostgreSQL override.
   Connection pool limits can be set with `RMM_DB_MAX_OPEN_CONNS` (default 16)
   and `RMM_DB_MAX_IDLE_CONNS` (default 4).

4. Verify `GET /healthz` returns `{"status":"ok"}` and check server logs for
   `existing SQLite data imported into PostgreSQL` on the first migrated start.
   Sign in, inspect devices and alerts, then download a database snapshot from
   the administration UI. PostgreSQL snapshots are custom-format `.dump` files;
   SQLite snapshots remain `.db` files. Preserve the original SQLite volume for
   rollback. Back up PostgreSQL dumps, the RMM key files and all three TLS volumes;
   the CA private key is needed to reissue the database certificate.

Do not run SQLite and PostgreSQL server instances against the same deployment
at the same time. The startup importer leaves a populated PostgreSQL database
unchanged; it does not synchronize later SQLite writes. Schema version and
checksum are checked at startup. A database with an unknown schema version
causes startup to fail instead of applying an unsafe change.

## Manual preflight and import

The bundled `rmm-db-migrate` command can validate a standalone SQLite snapshot
before cutover. If the stopped database has a non-empty WAL, use the recovery
copy made by the automatic importer or create a consistent SQLite snapshot
first; the manual command rejects a source with a non-empty WAL. Record the
snapshot's SHA-256, then run:

```sh
rmm-db-migrate \
  --source-sqlite=/readonly/rmm.db \
  --expected-source-sha256=<recorded-sha256> \
  --data-encryption-key-file=/readonly/data-encryption.key \
  --command-signing-key-file=/readonly/command-signing-ed25519.pem \
  --dry-run
```

To import manually into a separately reachable empty PostgreSQL database, set
`RMM_DATABASE_URL` and add `--confirm-cutover`. The integrated Compose setup
imports automatically. Use a read-only mount for the source and keys. The importer
checks source integrity and foreign keys, schema, data formats, key identity,
table counts and sensitive-data digests. It compares the source hash before
and after transfer. The JSON report includes counts, hashes and commit status.
An already populated target is rejected.

Development tests may use `sslmode=disable` only with a local PostgreSQL host
and explicit insecure development mode. Production startup requires
`sslmode=verify-full`.
