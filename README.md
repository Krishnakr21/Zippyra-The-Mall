# Zippyra — Retail Infrastructure Platform (v2.0)

A unified retail platform handling **100M+ customers** with real-time inventory, RFID validation, and AI-powered analytics. 

## Strict PDF Alignment (March 2026 Audit)
This codebase follows the **Zippyra Complete Development Guide v2.0**. All 18 microservices, 64 Kafka topics, and 6 Redis clusters are implemented per the mandated architecture.

## Repository Structure

```
├── platform/                    # Go Monorepo (github.com/zippyra/platform)
│   ├── services/                # 18 Microservices (Go 1.22)
│   │   ├── auth-service/        # :8080 (Primary)
│   │   ├── retailer-auth-service/ # :8094
│   │   ├── admin-auth-service/  # :8095
│   │   ├── cart-service/        # :8084
│   │   ├── catalog-service/     # :8083
│   │   ├── store-service/       # :8082
│   │   ├── order-service/       # :8086
│   │   ├── payment-service/     # :8085
│   │   ├── inventory-service/   # :8090
│   │   ├── compliance-service/  # :8098
│   │   └── ... (see Service Registry)
│   └── shared/                  # Shared Internal Packages
│       ├── b/                   # DB Pool (RDS Proxy compat)
│       ├── redis/               # 6-Cluster Routing + LUA
│       ├── kafka/               # 64 Topic constants + Partition Key
│       ├── jwt/                 # Ed25519 Asymmetric Auth
│       └── middleware/          # Standard chain
│
├── zippyra-customer-app/        # React Native (Expo SDK 51) + WatermelonDB
├── zippyra-retailer-dashboard/  # Next.js 14 App Router
├── zippyra-staff-app/           # React Native (Staff)
├── zippyra-kiosk/              # Electron Self-Service
├── zippyra-admin-platform/      # Turborepo (Admin + HQ)
│
├── database/                    # Schemas & Migrations
│   ├── postgresql/              # 46 Tables
│   ├── timescaledb/             # 8 Hypertables
│   └── clickhouse/              # 9 Analytics Tables
│
├── infra/                       # Terraform & K8s
│   ├── modules/                 # 11 Modules (vpc, rds, msk, eks, etc.)
│   └── environments/            # pilot, production
└── ...
```

## Quick Start (Monorepo)

```bash
cd platform
make help      # View all commands
make build-all # Build all 18 services
make docker-up # Start infrastructure
```

## Architectural Highlights
- **Zero PG in Happy Path**: Redis-only fast path for cart/scan.
- **256 Partitions**: On `cart.item_scanned` topic for massive scale.
- **Ed25519 Auth**: Asymmetric JWT validation across all surfaces.
- **Offline-First**: WatermelonDB v11 schema with custom migration logic.
