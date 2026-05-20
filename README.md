# nsw-agency

NSW Agency portal for the NSW (National Single Window) platform.

This repo contains both halves of the NSW Agency:

- [backend/](backend/) — Go service that holds NSW Agency-side application state, talks to the NSW core backend over OAuth2 M2M, and serves the frontend.
- [frontend/](frontend/) — React/Vite SPA used by NSW Agency officers (NPQS, FCAU, IRD, CDA) to review trader submissions.

The same codebase is deployed per NSW Agency, with branding and identity selected via env vars at build/runtime.

## Quick start

You need the NSW core backend and the Thunder/Asgardeo IdP running first. Both live in the [NSW monorepo](https://github.com/OpenNSW/nsw):

```bash
# In the NSW monorepo
cd nsw && make idp-up && make temporal-up && make backend
```

Then in this repo:

```bash
# Backend
cd backend
cp .env.example .env       # tweak OGA_NSW_* to point at your NSW backend + IdP
go run ./cmd/server

# Frontend (new terminal)
cd frontend
cp .env.example .env       # set VITE_IDP_CLIENT_ID, VITE_API_BASE_URL
pnpm install
pnpm dev
```

### Running a specific NSW Agency

Use [run-oga.sh](run-oga.sh) at the repo root to launch the per-NSW Agency backend and/or frontend with the right ports, DB file, IdP client id, and branding:

```bash
./run-oga.sh npqs              # backend + frontend for NPQS
./run-oga.sh fcau backend      # only backend for FCAU
./run-oga.sh ird frontend      # only frontend for IRD
./run-oga.sh cda               # backend + frontend for CDA
./run-oga.sh default           # generic branding/ports

# Fleet mode: bring up every NSW Agency at once
./run-oga.sh all               # all 4 backends (8081-8084) + frontends (5174-5177)
./run-oga.sh all backend       # only the backends
./run-oga.sh all frontend      # only the frontends
```

Every process runs in its own process group (`set -m`), so `Ctrl-C` cleanly stops the whole fleet — including the compiled binary `go run` spawns underneath. Logs from all processes interleave on the same terminal.

| NSW Agency | Backend port | DB file | NSW M2M client | Frontend port | Branding config | IdP client id |
|--------|--------------|---------|----------------|---------------|-----------------|---------------|
| NPQS | 8081 | `backend/npqs_applications.db` | `NPQS_TO_NSW` | 5174 | `frontend/public/configs/npqs.branding.json` | `OGA_PORTAL_APP_NPQS` |
| FCAU | 8082 | `backend/fcau_applications.db` | `FCAU_TO_NSW` | 5175 | `frontend/public/configs/fcau.branding.json` | `OGA_PORTAL_APP_FCAU` |
| IRD  | 8083 | `backend/ird_applications.db`  | `IRD_TO_NSW`  | 5176 | `frontend/public/configs/ird.branding.json`  | `OGA_PORTAL_APP_IRD` |
| CDA  | 8084 | `backend/cda_applications.db`  | `CDA_TO_NSW`  | 5177 | `frontend/public/configs/cda.branding.json`  | `OGA_PORTAL_APP_CDA` |

The branding-config paths above are gitignored — copy them from [default.branding.json](frontend/public/configs/default.branding.json) (see below).

The script sets `OGA_PORT`, `OGA_DB_PATH`, `OGA_NSW_CLIENT_ID` for the backend and `VITE_PORT`, `VITE_BRANDING_NAME`, `VITE_API_BASE_URL`, `VITE_IDP_CLIENT_ID`, `VITE_APP_URL` for the frontend. Any of these can be overridden by exporting them before invoking the script (other backend/frontend env vars — OAuth secrets, IdP base URL, etc. — still come from `backend/.env` and `frontend/.env`).

Only [default.branding.json](frontend/public/configs/default.branding.json) is tracked in git — per-NSW Agency branding files (`npqs.branding.json`, `fcau.branding.json`, `ird.branding.json`, `cda.branding.json`) are gitignored because branding is deployment-specific. To run a NSW Agency locally, copy the default and edit it:

```bash
cd frontend/public/configs
cp default.branding.json npqs.branding.json   # then edit appName, portalName, description, etc.
```

To add a brand-new NSW Agency, create `<name>.branding.json` the same way and add a matching `case` to [run-oga.sh](run-oga.sh).

## Prerequisites

### 1. NSW backend reachable

The NSW Agency calls the NSW core backend's `/api/v1/tasks` endpoint to return review results. Set `OGA_NSW_API_BASE_URL` in [backend/.env](backend/.env) accordingly (default: `http://localhost:8080/api/v1`).

### 2. M2M OAuth2 client

Each NSW Agency instance authenticates to NSW with its own M2M client. For local dev the IdP bootstrap creates a generic `OGA_TO_NSW` client; production deployments use NSW Agency-specific clients (`NPQS_TO_NSW`, etc.).

## Architecture

The NSW Agency is decoupled from the NSW core monorepo — it communicates over HTTP only:

```
trader-app → nsw-backend → (POST /api/oga/inject) → oga-backend ← oga-app
                  ▲                                       │
                  └────── (POST /api/v1/tasks, OAuth2 M2M)┘
```

- Own database (SQLite or PostgreSQL, per `OGA_DB_*` env vars) — not shared with NSW.
- Templates fetched from [OpenNSW/one-trade-templates](https://github.com/OpenNSW/one-trade-templates) at startup.
- No Temporal integration — the NSW Agency is a stateless HTTP microservice.

For details see [backend/docs/architecture.md](backend/docs/architecture.md).

## Releases

Tagging `vX.Y.Z` triggers [.github/workflows/release.yml](.github/workflows/release.yml), which builds and publishes:

- `ghcr.io/opennsw/nsw-agency/oga-backend:X.Y.Z`
- `ghcr.io/opennsw/nsw-agency/oga-app:X.Y.Z`

Image digests are bundled into a `release-digests.json` artifact attached to the GitHub Release.

## History

This repo was extracted from the [NSW monorepo](https://github.com/OpenNSW/nsw) `main` branch in May 2026 via `git-filter-repo`. Git history before that point reflects the monorepo paths (`oga/`, `portals/apps/oga-app/`).
