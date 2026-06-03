ARG HEADLESS=true
# HEADLESS=true  (default) — API-only image; the frontend build stage is
#                            unreachable and skipped by BuildKit entirely.
# HEADLESS=false            — OGA portal compiled via pnpm and embedded under
#                            /app/ui; the Go server serves it directly.

# Stage 1 — Build frontend (unreachable and skipped by BuildKit when HEADLESS=true)
FROM node:22-bookworm-slim AS frontend-builder

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
WORKDIR /app

RUN corepack enable

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./

RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile

COPY frontend/ .

RUN pnpm build

# Stage 2 — Build Go binary
FROM golang:1.25-bookworm AS go-builder

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
# -ldflags="-s -w" strips the symbol table and DWARF debug info to shrink the
# binaries. Go panic traces (function + line) and pprof still work, since they
# use the runtime pclntab rather than the symbol table.
RUN go build -ldflags="-s -w" -o /out/agency ./cmd/server \
  && go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# Stage 3 — Shared runtime base (Go binary + data, no UI)
FROM debian:bookworm-slim AS runtime-base

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && useradd -u 1001 -r -s /bin/false -d /app appuser \
  && mkdir -p /app /data \
  && chown -R appuser:appuser /data /app

WORKDIR /app

COPY --from=go-builder /out/agency /app/agency
COPY --from=go-builder /out/migrate /app/migrate
COPY --from=go-builder /app/migrations /app/migrations
COPY --from=go-builder /app/data/forms /app/data/forms
COPY --from=go-builder /app/data/task-configs /app/data/task-configs

# Stage 4a — API-only; selected when HEADLESS=true
FROM runtime-base AS final-true

# Stage 4b — Full image with embedded OGA frontend; selected when HEADLESS=false
FROM runtime-base AS final-false
COPY --chown=appuser:appuser --from=frontend-builder /app/dist /app/ui

# Stage 5 — Selector: resolves to final-true or final-false via the HEADLESS arg.
# ARG must be re-declared here because ARG scope resets at each FROM boundary.
ARG HEADLESS=true
FROM final-${HEADLESS}

LABEL org.opencontainers.image.source="https://github.com/OpenNSW/nsw-agency"
LABEL org.opencontainers.image.description="NSW Agency Backend Service"

USER appuser

EXPOSE 8081

CMD ["/app/agency"]
