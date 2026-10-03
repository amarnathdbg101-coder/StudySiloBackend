# Multi-stage production Dockerfile
# Stage 1: Build binary
FROM golang:alpine AS builder

# Enable automatic Go toolchain switching if required by go.mod
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install git and certificates
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build stripped binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/seed ./cmd/seed

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

# Copy ca-certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy compiled binaries
COPY --from=builder /app/bin/api /app/bin/api
COPY --from=builder /app/bin/migrate /app/bin/migrate
COPY --from=builder /app/bin/seed /app/bin/seed
COPY --from=builder /app/internal/migration /app/internal/migration
COPY --from=builder /app/.env.example /app/.env.example

EXPOSE 8080

CMD ["/app/bin/api"]