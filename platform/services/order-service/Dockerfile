FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /order-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /order-service /order-service
EXPOSE 8006
CMD ["/order-service"]
