FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /analytics-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /analytics-service /analytics-service
EXPOSE 8013
CMD ["/analytics-service"]
