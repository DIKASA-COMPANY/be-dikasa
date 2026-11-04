#!/usr/bin/env bash
# Runs only against the disposable CI PostgreSQL database, never a deployed database.
set -euo pipefail
: "${DATABASE_URL:?Disposable CI database URL is required}"
directory=$(mktemp -d)
trap 'rm -rf "$directory"' EXIT
export MIGRATIONS_DIR="$directory"
psql_test() { psql --dbname="$DATABASE_URL" -w -X -Atq -v ON_ERROR_STOP=1 "$@"; }
for file in migrations/*.up.sql; do
  # Match the Linux image's LF checkout even when verifying from a Windows checkout.
  migration_name=$(basename "$file")
  tr -d '\r' < "$file" > "$directory/$migration_name"
done
bash deploy/migrate.sh
bash deploy/migrate.sh
expected=$(find "$directory" -name '*.up.sql' | wc -l | tr -d ' ')
test "$(psql_test -c 'SELECT count(*) FROM public.dikasa_schema_migrations')" = "$expected"
psql_test -c 'DROP TABLE public.dikasa_schema_migrations'
if bash deploy/migrate.sh; then echo 'Legacy database was not rejected' >&2; exit 1; fi
last_file=$(find "$directory" -name '*.up.sql' | sort | tail -n 1)
last_version=$(basename "$last_file")
bash deploy/migrate.sh baseline "${last_version%%_*}"
bash deploy/migrate.sh
test "$(psql_test -c 'SELECT count(*) FROM public.dikasa_schema_migrations')" = "$expected"
printf '\n-- edited applied migration\n' >> "$directory/000001_create_identity_tables.up.sql"
if bash deploy/migrate.sh; then echo 'Drift was not rejected' >&2; exit 1; fi
tr -d '\r' < migrations/000001_create_identity_tables.up.sql > "$directory/000001_create_identity_tables.up.sql"
printf 'CREATE TABLE public.ci_migration_rollback (id integer);\n' > "$directory/999998_ci_transaction.up.sql"
printf 'SELECT ci_intentionally_missing_function();\n' > "$directory/999999_ci_failure.up.sql"
if bash deploy/migrate.sh; then echo 'Invalid migration was accepted' >&2; exit 1; fi
test "$(psql_test -c "SELECT to_regclass('public.ci_migration_rollback') IS NULL")" = t
test "$(psql_test -c 'SELECT count(*) FROM public.dikasa_schema_migrations')" = "$expected"
echo 'Migration replay, explicit legacy baseline, checksum drift and atomic rollback checks passed'
