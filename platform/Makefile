# Zippyra Platform Monorepo Makefile
# github.com/zippyra/platform

.PHONY: help build build-all test lint migrate-up seed docker-up

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build-all: ## Build all 18 microservices
	@for svc in services/*/; do \
		echo "Building $$(basename $$svc)..."; \
		go build -o /dev/null $$svc/cmd/server/main.go; \
	done

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run

migrate-up: ## Run PostgreSQL migrations
	# golang-migrate command here

seed: ## Seed development data
	# Seed logic here

docker-up: ## Start local dev infrastructure
	docker compose up -d

build-admin-cli: ## Build zippyra-admin
	go build -o zippyra-admin ./cmd/admin-cli/
