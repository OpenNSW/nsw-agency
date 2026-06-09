FROM golang:1.25-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# -ldflags="-s -w" strips the symbol table and DWARF debug info to shrink the
# binaries. Go panic traces (function + line) and pprof still work, since they
# use the runtime pclntab rather than the symbol table.
RUN go build -ldflags="-s -w" -o /out/agency ./cmd/server \
  && go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
  && go build -ldflags="-s -w" -o /out/seed ./cmd/seed

ARG NSW_SRILANKA_REF=main
RUN git clone --depth 1 --branch ${NSW_SRILANKA_REF} https://github.com/OpenNSW/nsw-srilanka /out/nsw-srilanka

FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/OpenNSW/nsw-agency"
LABEL org.opencontainers.image.description="NSW Agency Backend Service"

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && useradd -u 1001 -r -s /bin/false -d /app appuser \
  && mkdir -p /app /data \
  && chown -R appuser:appuser /data /app

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /out/agency /app/agency
COPY --from=builder --chown=appuser:appuser /out/migrate /app/migrate
COPY --from=builder --chown=appuser:appuser /out/seed /app/seed
COPY --from=builder --chown=appuser:appuser /app/migrations /app/migrations
COPY --from=builder --chown=appuser:appuser /app/data/task-configs /app/data/task-configs
COPY --from=builder --chown=appuser:appuser /out/nsw-srilanka/configs /app/nsw-srilanka-configs

ENV TASK_CONFIGS_DIR=/app/data/task-configs
ENV FORM_TEMPLATES_DIR=/app/nsw-srilanka-configs

# Runtime settings — UID 1001 matches appuser, OpenShift-compatible
USER appuser

# Default port (override via PORT env var)
EXPOSE 8081

CMD ["/app/agency"]
