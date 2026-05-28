# Build stage - compile per-platform inside buildx using automatic platform ARGs
# Pure Go sqlite (modernc.org/sqlite) means no CGO/glibc dependency
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

# Provided automatically by buildx for each target platform
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/dns-bench .

# Runtime stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates bash tzdata

WORKDIR /app
COPY --from=build /out/dns-bench ./dns-bench
COPY scripts/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# /results is where output files (CSV, HTML) will be written.
# Mount a PersistentVolumeClaim here in Kubernetes.
RUN mkdir -p /results

ENTRYPOINT ["/app/entrypoint.sh"]
