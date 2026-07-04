FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /warehouse-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /warehouse-service /warehouse-service
EXPOSE 8010
CMD ["/warehouse-service"]
