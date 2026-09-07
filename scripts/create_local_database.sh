#!/bin/sh
set -eu

POSTGRES_ADMIN_DSN="${POSTGRES_ADMIN_DSN:-postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable}"
POSTGRES_DB="${POSTGRES_DB:-food_ordering_api}"
POSTGRES_USER="${POSTGRES_USER:-foodapp}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-foodapp}"

psql "$POSTGRES_ADMIN_DSN" -v ON_ERROR_STOP=1 \
    -v db="$POSTGRES_DB" \
    -v user="$POSTGRES_USER" \
    -v password="$POSTGRES_PASSWORD" <<'SQL'
SELECT format('CREATE USER %I WITH PASSWORD %L', :'user', :'password')
WHERE NOT EXISTS (
    SELECT 1 FROM pg_roles WHERE rolname = :'user'
)\gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'db', :'user')
WHERE NOT EXISTS (
    SELECT 1 FROM pg_database WHERE datname = :'db'
)\gexec
SQL
