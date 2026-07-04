FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /loyalty-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /loyalty-service /loyalty-service
EXPOSE 8008
CMD ["/loyalty-service"]
