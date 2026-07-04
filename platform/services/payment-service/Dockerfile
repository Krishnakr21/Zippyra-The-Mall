FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy shared packages
COPY platform/shared /app/platform/shared

# Copy payment-service
COPY platform/services/payment-service /app/platform/services/payment-service

WORKDIR /app/platform/services/payment-service

# Explicitly fetch dependencies
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server/main.go

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/platform/services/payment-service/main .

EXPOSE 8086

CMD ["./main"]
