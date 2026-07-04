# Zippyra — Top-level Makefile
# Usage: make <target>

.PHONY: help dev stop build test lint migrate seed

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Development ────────────────────────────────────────────
dev: ## Start all local infrastructure (PG, Redis, Kafka, ES, CH)
	docker compose -f infrastructure/docker/docker-compose.dev.yaml up -d

stop: ## Stop all local infrastructure
	docker compose -f infrastructure/docker/docker-compose.dev.yaml down

# ─── Backend ────────────────────────────────────────────────
build: ## Build all backend services
	@for dir in backend/services/*/; do \
		echo "🔨 Building $$(basename $$dir)..."; \
		cd $$dir && CGO_ENABLED=0 go build -o /dev/null ./cmd/server && cd ../../..; \
	done
	@echo "✅ All services built successfully"

test: ## Run all backend tests
	@for dir in backend/services/*/; do \
		echo "🧪 Testing $$(basename $$dir)..."; \
		cd $$dir && go test ./... && cd ../../..; \
	done
	@echo "✅ All tests passed"

lint: ## Lint all Go code
	@cd backend && golangci-lint run ./...

# ─── Database ───────────────────────────────────────────────
migrate: ## Run all database migrations
	bash infrastructure/scripts/run-migrations.sh

seed: ## Seed development data
	bash infrastructure/scripts/seed-data.sh

# ─── Individual Services ────────────────────────────────────
run-auth: ## Run auth-service locally
	cd backend/services/auth-service && go run ./cmd/server

run-store: ## Run store-service locally
	cd backend/services/store-service && go run ./cmd/server

run-catalog: ## Run catalog-service locally
	cd backend/services/catalog-service && go run ./cmd/server

run-cart: ## Run cart-service locally
	cd backend/services/cart-service && go run ./cmd/server

run-inventory: ## Run inventory-service locally
	cd backend/services/inventory-service && go run ./cmd/server

run-order: ## Run order-service locally
	cd backend/services/order-service && go run ./cmd/server

run-payment: ## Run payment-service locally
	cd backend/services/payment-service && go run ./cmd/server

run-loyalty: ## Run loyalty-service locally
	cd backend/services/loyalty-service && go run ./cmd/server

run-exit: ## Run exit-validation-service locally
	cd backend/services/exit-validation-service && go run ./cmd/server

run-warehouse: ## Run warehouse-service locally
	cd backend/services/warehouse-service && go run ./cmd/server

run-notification: ## Run notification-service locally
	cd backend/services/notification-service && go run ./cmd/server

run-support: ## Run support-service locally
	cd backend/services/support-service && go run ./cmd/server

run-analytics: ## Run analytics-service locally
	cd backend/services/analytics-service && go run ./cmd/server

# ─── Mobile ─────────────────────────────────────────────────
mobile-install: ## Install mobile dependencies
	cd mobile && npm install

mobile-start: ## Start React Native Metro bundler
	cd mobile && npx react-native start

mobile-ios: ## Run mobile app on iOS simulator
	cd mobile && npx react-native run-ios

mobile-android: ## Run mobile app on Android emulator
	cd mobile && npx react-native run-android

# ─── Admin ──────────────────────────────────────────────────
admin-install: ## Install admin panel dependencies
	cd admin && npm install

admin-dev: ## Start admin panel dev server
	cd admin && npm run dev

admin-build: ## Build admin panel for production
	cd admin && npm run build
