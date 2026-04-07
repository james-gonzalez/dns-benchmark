# Runtime stage - binaries are pre-built by CI and copied in
# Pure Go sqlite (modernc.org/sqlite) means no CGO/glibc dependency
FROM alpine:3.23

ARG TARGETARCH=amd64

RUN apk --no-cache add ca-certificates bash tzdata

WORKDIR /app
COPY dns-bench-linux-${TARGETARCH} ./dns-bench
COPY scripts/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# /results is where output files (CSV, HTML) will be written.
# Mount a PersistentVolumeClaim here in Kubernetes.
RUN mkdir -p /results

ENTRYPOINT ["/app/entrypoint.sh"]
