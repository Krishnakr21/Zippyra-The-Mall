FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /support-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /support-service /support-service
EXPOSE 8012
CMD ["/support-service"]
