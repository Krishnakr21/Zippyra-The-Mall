# Store Service

The `store-service` provides essential retail infrastructure for Zippyra, including store management, real-time occupancy monitoring, and device tracking.

## 🚀 Getting Started

### 1. Configure the Environment
The service requires several environment variables for database, cache, and security. See the [Integrated Testing Guide](file:///Users/krishna/.gemini/antigravity/brain/a9e499e7-8879-4220-8ec4-5a277e5f3d8c/integrated_testing_guide.md) for a full list and instructions on generating JWT keys.

### 2. Run the Service
```bash
# Default port is 8081
go run ./cmd/server
```

### 3. Verification & Testing
- **Unit Tests**: `go test ./...`
- **Coverage Report**: See [walkthrough.md](file:///Users/krishna/.gemini/antigravity/brain/a9e499e7-8879-4220-8ec4-5a277e5f3d8c/walkthrough.md) for the 100% coverage summary.
- **Manual Testing**: Refer to the [Integrated Testing Guide](file:///Users/krishna/.gemini/antigravity/brain/a9e499e7-8879-4220-8ec4-5a277e5f3d8c/integrated_testing_guide.md) for cURL examples and SQL schema.

## 🛠 Tech Stack
- **Framework**: Go (Standard Library) + Chi Router
- **Database**: PostgreSQL (pgx)
- **Cache**: Redis
- **Message Bus**: Kafka
- **Security**: Ed25519 asymmetric JWT signing
- **Monitoring**: Sentry, Prometheus, OpenTelemetry
