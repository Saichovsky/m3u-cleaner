# --- Stage 1: Build Stage ---
FROM golang:1.22-alpine AS builder

# Only CA certificates are needed at runtime; the init container terminates
# after one run, so timezone data is intentionally omitted.
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum* ./
COPY main.go .

# Compile static binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o iptv-cleaner main.go

# --- Stage 2: Minimal Runtime Stage ---
FROM scratch

# Copy root CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy static binary
COPY --from=builder /app/iptv-cleaner /iptv-cleaner

# Default environment variables
ENV COUNTRIES="ke,uk,us"
ENV LISTDIR="/usr/share/nginx/html"

USER 10001:10001

# Log timestamps are UTC by design (image has no tzdata).
ENTRYPOINT ["/iptv-cleaner"]