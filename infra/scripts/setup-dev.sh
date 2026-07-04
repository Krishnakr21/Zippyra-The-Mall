#!/bin/bash
set -e
echo "🚀 Setting up Zippyra development environment..."
echo "1. Starting Docker containers..."
docker compose -f infrastructure/docker/docker-compose.dev.yaml up -d
echo "2. Running database migrations..."
# bash infrastructure/scripts/run-migrations.sh
echo "3. Seeding development data..."
# bash infrastructure/scripts/seed-data.sh
echo "✅ Development environment ready!"
echo "   PostgreSQL:    localhost:5432"
echo "   Redis:         localhost:6379"
echo "   Kafka:         localhost:9092"
echo "   ElasticSearch: localhost:9200"
echo "   ClickHouse:    localhost:8123"
