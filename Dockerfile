# Stage 1: Build
# CGO is required for the browser/sqlite3 package.
# We use a Debian-based image to have access to gcc.
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with CGO enabled (required for go-sqlite3)
# The browser feature is not used in Kubernetes, but we keep CGO_ENABLED=1
# so the binary is built consistently. Strip debug info to reduce size.
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o dns-bench .

# Stage 2: Runtime
# Use a minimal Debian image (not scratch/alpine) because CGO requires glibc.
FROM debian:bookworm-slim

# Install CA certificates (for DoH HTTPS) and bash (for entrypoint script)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary and entrypoint script from the builder stage
COPY --from=builder /app/dns-bench /app/dns-bench
COPY scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# /results is where output files (CSV, HTML) will be written.
# Mount a PersistentVolumeClaim here in Kubernetes.
RUN mkdir -p /results

ENTRYPOINT ["/app/entrypoint.sh"]
