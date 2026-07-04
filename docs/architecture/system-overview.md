# Zippyra System Overview

## Architecture Pattern
Event-driven microservices with CQRS (Command Query Responsibility Segregation).

## Data Stores
- **PostgreSQL 16**: OLTP — 46 tables across 12 service groups
- **TimescaleDB**: Time-series — 8 hypertables, 15 continuous aggregates
- **ClickHouse**: OLAP — analytics/ML, 9 tables
- **Redis**: Cart state, product availability, gateway health, store capacity
- **ElasticSearch**: Product catalog search (64 shards, store_id routing)
- **WatermelonDB**: Offline-first mobile, 11 tables

## Scale Targets
- 100M registered users | 1.2B scans/day | 40M orders/day | 14K msgs/sec Kafka
