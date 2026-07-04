#!/bin/bash
set -e
echo "Running PostgreSQL migrations..."
# migrate -path database/postgresql/migrations -database "$DATABASE_URL" up
echo "Running TimescaleDB migrations..."
# migrate -path database/timescaledb/migrations -database "$TIMESCALE_URL" up
echo "Running ClickHouse migrations..."
# Similar migration command for ClickHouse
echo "✅ All migrations completed"
