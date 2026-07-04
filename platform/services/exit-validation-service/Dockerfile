FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /exit-validation-service ./cmd/server
FROM alpine:3.19
COPY --from=builder /exit-validation-service /exit-validation-service
EXPOSE 8009
CMD ["/exit-validation-service"]
