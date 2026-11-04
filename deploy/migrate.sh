#!/bin/sh
# Apply immutable SQL files once, in one transaction protected by a database lock.
set -eu
: "${DATABASE_URL:?DATABASE_URL is required}"
directory=${MIGRATIONS_DIR:-/app/migrations}
mode=${1:-up}
baseline=${2:-}
case "$mode" in
  up) ;;
  baseline) case "$baseline" in ??????) ;; *) echo 'Supply a six-digit baseline version' >&2; exit 1 ;; esac
    case "$baseline" in *[!0-9]*) exit 1 ;; esac ;;
  *) echo 'Usage: migrate.sh [up | baseline 000018]' >&2; exit 1 ;;
esac
test -d "$directory"
sql=$(mktemp)
trap 'rm -f "$sql"' EXIT HUP INT TERM
cat > "$sql" <<'SQL'
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL lock_timeout = '60s';
SELECT pg_advisory_xact_lock(614724851);
CREATE TABLE IF NOT EXISTS public.dikasa_schema_migrations (
  name text PRIMARY KEY,
  sha256 text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
);
SQL
if [ "$mode" = up ]; then
  cat >> "$sql" <<'SQL'
DO $$ BEGIN
  IF to_regclass('public.users') IS NOT NULL
     AND NOT EXISTS (SELECT 1 FROM public.dikasa_schema_migrations) THEN
    RAISE EXCEPTION 'Existing database requires an explicitly reviewed migration baseline';
  END IF;
END $$;
SQL
else
  # Baseline is an operator-only action, never selected by the deployment workflow.
  cat >> "$sql" <<'SQL'
DO $$ BEGIN
  IF to_regclass('public.users') IS NULL THEN
    RAISE EXCEPTION 'Cannot baseline an empty database';
  END IF;
END $$;
SQL
  test -f "$(find "$directory" -maxdepth 1 -name "${baseline}_*.up.sql" -print -quit)"
fi
for file in "$directory"/*.up.sql; do
  test -f "$file"
  name=$(basename "$file")
  # Only repository migration filenames may be interpolated into SQL literals.
  case "$name" in *[!a-z0-9_.]*) echo 'Invalid migration filename' >&2; exit 1 ;; esac
  version=${name%%_*}
  if [ "$mode" = baseline ] && [ "$version" -gt "$baseline" ]; then continue; fi
  checksum=$(sha256sum "$file" | cut -d ' ' -f 1)
  cat >> "$sql" <<SQL
DO \$\$ BEGIN
  IF EXISTS (SELECT 1 FROM public.dikasa_schema_migrations
             WHERE name = '$name' AND sha256 <> '$checksum') THEN
    RAISE EXCEPTION 'Migration checksum mismatch: $name';
  END IF;
END \$\$;
SELECT EXISTS (SELECT 1 FROM public.dikasa_schema_migrations WHERE name = '$name') AS applied \gset
\if :applied
\else
SQL
  if [ "$mode" = up ]; then cat "$file" >> "$sql"; printf '\n' >> "$sql"; fi
  printf "INSERT INTO public.dikasa_schema_migrations (name, sha256) VALUES ('%s', '%s');\n" "$name" "$checksum" >> "$sql"
  printf '\\endif\n' >> "$sql"
done
printf 'COMMIT;\n' >> "$sql"
# Pass the URI explicitly: PGDATABASE alone is not expanded as a connection URI.
# Never echo the DSN or SQL; fail instead of prompting for missing credentials.
psql --dbname="$DATABASE_URL" -w -X -q -v ON_ERROR_STOP=1 -f "$sql"
