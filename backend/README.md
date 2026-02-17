# OGA Portal Backend

A standalone Go microservice that acts as a verification hub for external government agencies within the [NSW (National Single Window)](../README.md) trade facilitation platform. It enables the NSW core service to inject data for review, supports configurable dynamic forms per agency, and sends callback responses to the originating service upon review completion.

## How It Fits Into NSW

The OGA service is an implementation of the **OGA Service Module (OGA SM)** described in the NSW architecture. It embodies the "state vs. data decoupling" principle:

- **NSW Core Workflow Engine (CWE)** manages process state (e.g., "Waiting for Approval")
- **OGA Service Module** manages domain data (e.g., phytosanitary inspection details)

Each government agency runs its own OGA instance with its own database, ensuring data isolation and sovereignty.

```
┌─────────────────┐         POST /api/oga/inject           ┌──────────────────┐
│                 │ ──────────────────────────────────────▶│                  │
│  NSW Core       │                                        │   OGA Service    │
│    Service      │◀────────────────────────────────────── │   (per agency)   │
│                 │     POST {serviceUrl} (callback)       │                  │
└─────────────────┘                                        └──────────────────┘
                                                                   │
                                                            ┌──────┴──────┐
                                                            │  SQLite DB  │
                                                            │ (per agency)│
                                                            └─────────────┘
```

## Features

- **Data Injection** – External services POST data for OGA review via `/api/oga/inject`
- **Dynamic Review Forms** - Metadata-driven form selection using [JSON Forms](https://jsonforms.io/) schema
- **Paginated Listings** – Fetch applications with status filtering and pagination
- **Review Workflow** - Approve/Reject with configurable reviewer response fields
- **Callback Responses** – Automatically POSTs review results back to the originating service
- **Per-Agency Isolation** – Each agency instance has its own database and port
- **Graceful Shutdown** -- Signal-based shutdown with in-flight request draining

## Getting Started

### Prerequisites

- Go 1.25+
- GCC (required by `go-sqlite3` CGO dependency)

### Run Locally

```bash
cd oga

# Run with defaults (port 8081, SQLite at ./oga_applications.db)
go run ./cmd/server

# Run with custom config
OGA_PORT=8081 OGA_DB_PATH=./npqs.db go run ./cmd/server
```

The database is auto-created and auto-migrated on first startup.

### Build

```bash
go build -o bin/oga ./cmd/server
./bin/oga
```

### Running Multiple Agency Instances

Each agency should run as a separate instance:

```bash
# Terminal 1 -- NPQS (National Plant Quarantine Service)
OGA_PORT=8081 OGA_DB_PATH=./npqs_applications.db go run ./cmd/server

# Terminal 2 -- FCAU (Food Control Administration Unit)
OGA_PORT=8082 OGA_DB_PATH=./fcau_applications.db go run ./cmd/server
```

### Configuration

All configuration is via environment variables:

| Variable              | Description                             | Default                 |
|-----------------------|-----------------------------------------|-------------------------|
| `OGA_PORT`            | HTTP server port                        | `8081`                  |
| `OGA_DB_PATH`         | Path to SQLite database file            | `./oga_applications.db` |
| `OGA_FORMS_PATH`      | Directory containing form JSON files    | `./data/forms`          |
| `OGA_DEFAULT_FORM_ID` | Fallback form ID when no metadata match | `default`               |

See [`.env.example`](.env.example) for a template.

## API Reference

See [docs/api.md](docs/api.md) for complete API documentation with request/response examples.

**Quick overview:**

| Method | Endpoint                                | Description                                |
|--------|-----------------------------------------|--------------------------------------------|
| `GET`  | `/health`                               | Health check                               |
| `POST` | `/api/oga/inject`                       | Inject data for review (called by NSW)     |
| `GET`  | `/api/oga/applications`                 | List applications (paginated, filterable)  |
| `GET`  | `/api/oga/applications/{taskId}`        | Get single application with review form    |
| `POST` | `/api/oga/applications/{taskId}/review` | Submit review decision (triggers callback) |

## Documentation

Detailed documentation lives in the [`docs/`](docs/) folder:

| Document                                   | Description                                        |
|--------------------------------------------|----------------------------------------------------|
| [Architecture](docs/architecture.md)       | System design, layered architecture, data flow     |
| [API Reference](docs/api.md)               | Complete endpoint docs with examples               |
| [Dynamic Forms](docs/dynamic-forms.md)     | How the form system works and how to add new forms |
| [NSW Integration](docs/nsw-integration.md) | How OGA connects to the NSW workflow engine        |

## Project Structure

```
oga/
├── cmd/server/
│   └── main.go                 # Entry point, server setup, graceful shutdown
├── internal/
│   ├── config.go               # Environment-based configuration
│   ├── handler.go              # HTTP handlers for all endpoints
│   ├── service.go              # Business logic, callback dispatch
│   ├── store.go                # GORM + SQLite database operations
│   ├── form.go                 # Form store -- loads and serves form definitions
│   └── utils.go                # JSON response helpers
├── data/forms/                 # Review form definitions (JSON Forms format)
│   ├── default.json            # Generic approval/rejection form
│   └── consignment:moa:npqs:phytosanitary:001.json
├── docs/                       # Documentation
├── .env.example                # Example environment configuration
├── go.mod
└── go.sum
```

## Contributing

Please read the project-level [CONTRIBUTING.md](../docs/CONTRIBUTING.md) before submitting changes.

When working on the OGA module:

1. All application code lives in `internal/` (unexported package)
2. Form definitions go in `data/forms/` as JSON files
3. The service uses Go's standard `net/http` with `http.ServeMux` -- no external routing frameworks
4. Database migrations are handled automatically by GORM's `AutoMigrate`
5. Run `go vet ./...` and `go build ./...` before submitting PRs

## License

Distributed under the Apache 2.0 License. See [LICENSE](../LICENSE) for more information.