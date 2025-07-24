# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build and Run
- `go build -o event-service ./cmd/event-service` - Build the service
- `go run ./cmd/event-service` - Run without building (requires environment variables)
- `./event-service` - Run the built binary

### Testing
- `make test` - Run all tests with race detection
- `make test-nocache` - Run tests without cache
- `make test-bench` - Run benchmarks
- `go test ./service/domain/...` - Run tests for a specific package
- `go test -run TestNameOfType_ExpectedBehaviour` - Run a specific test

### Linting and Formatting
- `make fmt` - Format code using gosimports
- `make lint` - Run go vet and golangci-lint
- `make tools` - Install required tools (linter, formatter)

### Development Workflow
- `make ci` - Run full CI pipeline (test, bench, lint, generate, fmt, tidy)
- `make tidy` - Update go.mod dependencies
- `make generate` - Run go generate (updates Wire dependency injection)

### Running Locally
```bash
EVENTS_DATABASE_PATH=/path/to/database.sqlite \
EVENTS_ENVIRONMENT=DEVELOPMENT \
go run ./cmd/event-service
```

## Architecture Overview

nos-event-service is a Go service that crawls Nostr relays to replicate events relevant to Nos users and distributes them to other services.

### Service Layer Structure
```
service/
├── app/          # Application handlers and use cases
├── domain/       # Core business logic and entities
├── adapters/     # External service implementations (relays, pubsub, metrics)
└── ports/        # Interfaces for external dependencies
```

### Core Components

1. **Downloader**: Manages connections to Nostr relays and downloads events based on filters
2. **Event Processing Pipeline**:
   - `SaveReceivedEventHandler` - Persists incoming events to SQLite
   - `ProcessSavedEventHandler` - Asynchronously processes saved events
3. **Relay Management**: Discovers relays from event data and bootstrap sources
4. **Contact Management**: Tracks Nos users and their contact lists for filtering
5. **Internal SQLite Pubsub**: Custom out-of-order message processing between handlers

### Event Filtering Logic
The service downloads:
- Global event kinds (defined in `globalEventKindsToDownload`)
- All events from Nos users and their contacts
- Events mentioning Nos users (via `p` tags)

### Key Dependencies
- **Database**: SQLite with migrations in `migrations/`
- **Dependency Injection**: Google Wire (see `wire.go` files)
- **Messaging**: Watermill abstractions with custom SQLite implementation
- **Nostr Protocol**: `github.com/nbd-wtf/go-nostr`
- **Metrics**: Prometheus with custom metrics in `service/adapters/prometheus`

### Environment Configuration
Required:
- `EVENTS_DATABASE_PATH` - Full path to SQLite database

Development vs Production:
- `EVENTS_ENVIRONMENT=DEVELOPMENT` - Disables Google Cloud Pubsub
- `EVENTS_ENVIRONMENT=PRODUCTION` - Requires Google Cloud credentials

### Testing Conventions
- Test files alongside source (`*_test.go`)
- Test naming: `TestTypeName_ExpectedBehavior`
- Uses `stretchr/testify` for assertions
- Race detection enabled by default
- Mock interfaces defined in `_mocks.go` files