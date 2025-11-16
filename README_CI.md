# Continuous Integration (CI) Setup

This project uses GitHub Actions for automated testing and quality assurance.

## Test Types

### Unit Tests (Fast)
- Run on every push and pull request
- Test individual functions with mocks
- Includes race condition detection
- Execution time: ~2 seconds

```bash
go test ./...

# With race detection (CI also runs this)
go test -race -short ./...
```

### Integration Tests (Slow)
- Run on pull requests to `main` or `develop` branches and pushes to `main`/`develop`
- Test real database operations using PostgreSQL containers
- Uses testcontainers-go for automated database setup
- Execution time: ~4 seconds

```bash
go test -tags=integration ./...
```

### Code Quality
- **Linting**: `golangci-lint` checks code style and potential issues
- **Building**: Ensures code compiles successfully and builds server binary
- **Race Detection**: Tests for race conditions in unit tests

## Local Development

### Run All Tests
```bash
# Unit tests only
go test ./...

# All tests (including integration)
go test -tags=integration ./...
```

### Run Specific Test Packages
```bash
# API tests
go test ./internal/api

# Store tests
go test ./internal/store

# Integration tests only
go test -tags=integration ./internal/store
```

### Code Quality Checks
```bash
# Lint code
golangci-lint run

# Build project
go build ./...

# Run with race detector
go test -race ./...
```

## CI Workflow

The GitHub Actions workflow (`.github/workflows/go.yml`) includes:

1. **unit-tests**: Fast unit tests on every push/PR (includes race detection)
2. **integration-tests**: Full integration tests on PRs to main/develop branches and pushes to main/develop
3. **build**: Compilation check and binary building
4. **lint**: Code quality analysis with golangci-lint

## Prerequisites for Local Integration Tests

- Docker installed and running
- PostgreSQL 15-alpine available (via Docker)

The integration tests use `testcontainers-go` to automatically spin up PostgreSQL containers.

## CI Features

The CI pipeline includes several optimizations:

- **Go Module Caching**: Speeds up dependency downloads
- **Docker Service Integration**: PostgreSQL service for integration tests
- **Binary Building**: CI builds the server binary to ensure it compiles
- **Parallel Jobs**: Unit tests, integration tests, build, and lint run in parallel
- **Conditional Execution**: Integration tests only run on significant changes