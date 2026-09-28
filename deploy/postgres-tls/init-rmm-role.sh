#!/bin/sh
set -e

: "${RMM_POSTGRES_PASSWORD:?RMM_POSTGRES_PASSWORD is required}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
\getenv app_password RMM_POSTGRES_PASSWORD
CREATE ROLE rmm LOGIN PASSWORD :'app_password';
GRANT CONNECT ON DATABASE rmm TO rmm;
GRANT USAGE, CREATE ON SCHEMA public TO rmm;
SQL
