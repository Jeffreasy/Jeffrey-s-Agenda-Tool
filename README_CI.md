# Continuous Integration (CI) Setup

This project uses GitHub Actions for automated testing and quality assurance.

## Test Types

### Unit Tests (Fast)
- Run on every push and pull request
- Test individual functions with mocks
- Execution time: ~2 seconds

```bash
go test ./...
```

### Integration Tests (Slow)
- Run only on pull requests to `main` or `develop` branches
- Test real database operations using PostgreSQL containers
- Execution time: ~4 seconds

```bash
go test -tags=integration ./...
```

### Code Quality
- **Linting**: `golangci-lint` checks code style and potential issues
- **Building**: Ensures code compiles successfully
- **Race Detection**: Tests for race conditions

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

1. **unit-tests**: Fast unit tests on every push/PR
2. **integration-tests**: Full integration tests on PRs to main/develop
3. **build**: Compilation check
4. **lint**: Code quality analysis

## Prerequisites for Local Integration Tests

- Docker installed and running
- PostgreSQL 15+ available (via Docker)

The integration tests use `testcontainers-go` to automatically spin up PostgreSQL containers.