FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /notification-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /notification-service /notification-service
EXPOSE 8011
CMD ["/notification-service"]
