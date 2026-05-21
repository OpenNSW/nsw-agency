# NSW Agency Portal Backend

A standalone Go microservice that acts as a verification hub for external agencies within the [NSW (National Single Window)](../README.md) trade facilitation platform. It enables the NSW core service to inject data for review, supports configurable dynamic forms per agency, and sends callback responses to the originating service upon review completion.

## How It Fits Into NSW

The NSW Agency service is an implementation of the **NSW Agency Service Module (NSW Agency SM)** described in the NSW architecture. It embodies the "state vs. data decoupling" principle:

- **NSW Core Workflow Engine (CWE)** manages process state (e.g., "Waiting for Approval")
- **NSW Agency Service Module** manages domain data (e.g., phytosanitary inspection details)

Each government agency runs its own NSW Agency instance with its own database, ensuring data isolation and sovereignty.

```
┌─────────────────┐         POST /api/nsw-agency/inject           ┌──────────────────┐
│                 │ ──────────────────────────────────────▶│                  │
│  NSW Core       │                                        │   NSW Agency Service    │
│    Service      │◀────────────────────────────────────── │   (per agency)   │
│                 │     POST {serviceUrl} (callback)       │                  │
└─────────────────┘                                        └──────────────────┘
                                                                   │
                                                            ┌──────┴──────┐
                                                            │  DB Store   │
                                                            │ (SQLite/PG) │
                                                            └─────────────┘
```

## Features

- **Data Injection** – External services POST data for NSW Agency review via `/api/nsw-agency/inject`
- **Task Configurations** – Per-taskCode metadata (title, icon, category), form references, and outcome-to-status mapping
- **Dynamic Forms** – Reusable [JSON Forms](https://jsonforms.io/) definitions referenced by ID from task configs
- **Paginated Listings** – Fetch applications with status filtering and pagination
- **Review Workflow** – Approve/Reject driven by configurable status maps
- **Callback Responses** – Automatically POSTs review results back to the originating service
- **Per-agency Isolation** – Each NSW Agency instance has its own database and port
- **Graceful Shutdown** -- Signal-based shutdown with in-flight request draining

## Getting Started

### Prerequisites

- Go 1.25+
- GCC (required by `go-sqlite3` CGO dependency)

### Run Locally

```bash
cd backend

# Run with defaults (port 8081, SQLite at ./nsw_agency_applications.db)
go run ./cmd/server

# Run with custom config (SQLite)
NSW_AGENCY_PORT=8081 NSW_AGENCY_DB_PATH=./npqs.db go run ./cmd/server

# Run with custom config (PostgreSQL)
NSW_AGENCY_DB_DRIVER=postgres NSW_AGENCY_DB_NAME=npqs_db NSW_AGENCY_DB_USER=postgres NSW_AGENCY_DB_PASSWORD=changeme go run ./cmd/server
```

The database is auto-created and auto-migrated on first startup.

### Build

```bash
go build -o bin/nsw-agency ./cmd/server
./bin/nsw-agency
```

### Running Multiple NSW Agency Instances

Each NSW Agency should run as a separate instance:

```bash
# Terminal 1 -- NPQS (National Plant Quarantine Service)
NSW_AGENCY_PORT=8081 NSW_AGENCY_DB_PATH=./npqs_applications.db go run ./cmd/server

# Terminal 2 -- FCAU (Food Control Administration Unit)
NSW_AGENCY_PORT=8082 NSW_AGENCY_DB_PATH=./fcau_applications.db go run ./cmd/server
```

### Configuration

All configuration is via environment variables:

| Variable                             | Description                                            | Default                        |
|--------------------------------------|--------------------------------------------------------|--------------------------------|
| `NSW_AGENCY_PORT`                           | HTTP server port                                       | `8081`                         |
| `NSW_AGENCY_DB_DRIVER`                      | Database driver (`sqlite`, `postgres`)                 | `sqlite`                       |
| `NSW_AGENCY_DB_PATH`                        | Path to SQLite database file                           | `./nsw_agency_applications.db`        |
| `NSW_AGENCY_DB_HOST`                        | PostgreSQL host                                        | `localhost`                    |
| `NSW_AGENCY_DB_PORT`                        | PostgreSQL port                                        | `5432`                         |
| `NSW_AGENCY_DB_USER`                        | PostgreSQL user                                        | `postgres`                     |
| `NSW_AGENCY_DB_PASSWORD`                    | PostgreSQL password                                    | `changeme`                     |
| `NSW_AGENCY_DB_NAME`                        | PostgreSQL database name                               | `nsw_agency_db`                       |
| `NSW_AGENCY_DB_SSLMODE`                     | PostgreSQL SSL mode                                    | `disable`                      |
| `NSW_AGENCY_CONFIG_DIR`                     | Root directory containing `task-configs/` and `forms/` | `./data`                       |
| `NSW_AGENCY_DEFAULT_TASK_CONFIG_ID`         | Fallback task config ID when `taskCode` has no match   | `default`                      |
| `NSW_AGENCY_ALLOWED_ORIGINS`                | Comma-separated CORS origins (`*` to allow all)        | `*`                            |
| `NSW_AGENCY_NSW_API_BASE_URL`               | NSW API base URL for calling NSW endpoints             | `http://localhost:8080/api/v1` |
| `NSW_AGENCY_NSW_CLIENT_ID`                  | OAuth2 client ID for NSW Agency -> NSW                        | required                       |
| `NSW_AGENCY_NSW_CLIENT_SECRET`              | OAuth2 client secret for NSW Agency -> NSW                    | required                       |
| `NSW_AGENCY_NSW_TOKEN_URL`                  | OAuth2 token endpoint URL                              | required                       |
| `NSW_AGENCY_NSW_SCOPES`                     | Optional comma-separated OAuth2 scopes                 | empty                          |
| `NSW_AGENCY_NSW_TOKEN_INSECURE_SKIP_VERIFY` | DEV-only: skip TLS verification for token fetch        | `false`                        |

See [`.env.example`](.env.example) for a template.

## API Reference

See [docs/api.md](docs/api.md) for complete API documentation with request/response examples.

**Quick overview:**

| Method | Endpoint                                | Description                                |
|--------|-----------------------------------------|--------------------------------------------|
| `GET`  | `/health`                               | Health check                               |
| `POST` | `/api/nsw-agency/inject`                       | Inject data for review (called by NSW)     |
| `GET`  | `/api/nsw-agency/applications`                 | List applications (paginated, filterable)  |
| `GET`  | `/api/nsw-agency/applications/{taskId}`        | Get single application with review form    |
| `POST` | `/api/nsw-agency/applications/{taskId}/review` | Submit review decision (triggers callback) |

## Documentation

Detailed documentation lives in the [`docs/`](docs/) folder:

| Document                                    | Description                                                                                |
|---------------------------------------------|--------------------------------------------------------------------------------------------|
| [Architecture](docs/architecture.md)        | System design, layered architecture, data flow                                             |
| [API Reference](docs/api.md)                | Complete endpoint docs with examples                                                       |
| [Task Configurations](docs/task-configs.md) | Per-taskCode metadata, form references, and status-mapping behavior; how to add a new task |
| [Forms](docs/forms.md)                      | JSON Forms file structure and how to add new forms referenced from task configs            |
| [NSW Integration](docs/nsw-integration.md)  | How NSW Agency connects to the NSW workflow engine                                                |

## Project Structure

```
backend/
├── cmd/server/
│   └── main.go                 # Entry point, server setup, graceful shutdown
├── internal/
│   ├── config.go               # Environment-based configuration
│   ├── handler.go              # HTTP handlers for all endpoints
│   ├── service.go              # Business logic, callback dispatch
│   ├── store.go                # GORM-based application repository
│   ├── task_config.go          # TaskConfigStore -- per-taskCode UI metadata and form refs
│   ├── form.go                 # FormStore -- pure JSON Forms definitions
│   ├── utils.go                # JSON response helpers
│   ├── database/               # Driver setup and connection (SQLite + PostgreSQL)
│   ├── feedback/               # Trader feedback endpoint
│   └── storage/                # Upload/download URL handling for file attachments
├── pkg/
│   ├── httpclient/             # OAuth2-aware HTTP client used for outbound NSW calls
│   └── httputil/               # Shared HTTP helpers
├── data/                       # Local config dir (gitignored; only defaults are tracked)
│   ├── task-configs/
│   │   └── default.json        # Default fallback task config (shipped in repo)
│   └── forms/
│       └── default_review.json # Default review form (shipped in repo)
├── docs/                       # Documentation
├── Dockerfile
├── workload.yaml
├── .env.example                # Example environment configuration
├── go.mod
└── go.sum
```

## Contributing

Please read the project-level [CONTRIBUTING.md](../docs/CONTRIBUTING.md) before submitting changes.

When working on the NSW Agency module:

1. All application code lives in `internal/` (unexported package)
2. Task configs go in `data/task-configs/` and form definitions go in `data/forms/`. See [`docs/task-configs.md`](docs/task-configs.md) and [`docs/forms.md`](docs/forms.md).
3. The service uses Go's standard `net/http` with `http.ServeMux` -- no external routing frameworks
4. Database migrations are handled automatically by GORM's `AutoMigrate`
5. Run `go vet ./...` and `go build ./...` before submitting PRs

## License

Distributed under the Apache 2.0 License. See [LICENSE](../LICENSE) for more information.
