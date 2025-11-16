# Testing Documentation

This document provides comprehensive information about the testing strategy, test types, and testing practices used in the Agenda Automator backend.

## Overview

The Agenda Automator backend employs a comprehensive testing strategy that includes unit tests, integration tests, and code quality checks. The testing suite is designed to ensure reliability, maintainability, and correctness of the dual-service automation platform (Google Calendar + Gmail).

## Test Types

### 1. Unit Tests

Unit tests focus on testing individual functions and components in isolation using mocks and stubs.

#### Characteristics
- **Fast execution**: Typically complete in seconds
- **Isolated**: No external dependencies (database, network calls)
- **Mock-based**: Uses mock implementations for external services
- **Coverage-focused**: Aim for high code coverage (>80%)

#### Test Files
- `internal/api/health/health_test.go` - Health endpoint testing
- `internal/api/user/user_test.go` - User management API testing
- `internal/api/auth/auth_test.go` - Authentication flow testing
- `internal/api/account/account_test.go` - Connected accounts API testing
- `internal/store/store_test.go` - Store layer unit tests
- `internal/database/database_test.go` - Database connection testing
- `internal/worker/worker_test.go` - Worker logic testing

#### Example Unit Test
```go
func TestHandleHealth(t *testing.T) {
    testLogger := zap.NewNop()

    req, err := http.NewRequest("GET", "/api/v1/health", http.NoBody)
    assert.NoError(t, err)

    rr := httptest.NewRecorder()
    handler := HandleHealth(testLogger)
    handler.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusOK, rr.Code)

    var response map[string]string
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Equal(t, "ok", response["status"])
}
```

#### Running Unit Tests
```bash
# Run all unit tests
go test ./...

# Run with race detection
go test -race -short ./...

# Run specific package
go test ./internal/api/health

# Run with verbose output
go test -v ./...
```

### 2. Integration Tests

Integration tests verify that different components work together correctly, including real database operations.

#### Characteristics
- **Real dependencies**: Uses actual PostgreSQL database via testcontainers
- **Full stack**: Tests complete workflows from API to database
- **Build tag protected**: Uses `//go:build integration` to separate from unit tests
- **Container-based**: Automatically spins up PostgreSQL containers

#### Test Files
- `internal/store/integration_test.go` - Full database integration testing
- `internal/database/migrations_test.go` - Database migration testing

#### Example Integration Test
```go
//go:build integration

func TestDatabaseIntegration(t *testing.T) {
    ctx := context.Background()
    logger := zap.NewNop()

    // Start PostgreSQL container
    pgContainer, err := postgres.Run(ctx,
        "postgres:15-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("testuser"),
        postgres.WithPassword("testpass"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second)),
    )
    require.NoError(t, err)
    defer pgContainer.Terminate(context.Background())

    // Get connection and test operations
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)
    defer pool.Close()

    // Test migrations, table creation, and CRUD operations
    // ... test code ...
}
```

#### Running Integration Tests
```bash
# Run integration tests (requires Docker)
go test -tags=integration ./...

# Run with verbose output
go test -tags=integration -v ./...

# Run specific integration test
go test -tags=integration ./internal/store -run TestDatabaseIntegration
```

### 3. Code Quality Tests

Code quality is enforced through automated linting and static analysis.

#### golangci-lint Configuration

The project uses golangci-lint with a carefully tuned configuration (`.golangci.yml`):

```yaml
linters:
  disable-all: true
  enable:
    - unused        # Detects unused code
    - revive        # Modern golint replacement
    - copyloopvar   # Prevents loop variable issues
    - errcheck      # Ensures error handling
    - govet         # Go's built-in static analyzer
    - staticcheck   # Advanced static analysis
    - goimports     # Import organization
    - gofmt         # Code formatting
    - goconst       # Magic number detection
    - gocritic      # Code improvements
    - gosec         # Security issues
    - noctx         # Context usage validation
```

#### Excluded Rules
Some rules are excluded for practical reasons:
- `noctx` in test files (HTTP requests in tests)
- `errcheck` for `log.Sync()` calls
- `revive` stutter and var-naming rules for API consistency

#### Running Code Quality Checks
```bash
# Run all linters
golangci-lint run

# Run with specific timeout
golangci-lint run --timeout=5m

# Run on specific files
golangci-lint run ./internal/api/...

# Fix auto-fixable issues
golangci-lint run --fix
```

## Mock Implementations

The testing suite includes comprehensive mock implementations for external dependencies.

### Mock Store
- **File**: `internal/store/mock_store.go`
- **Purpose**: Mocks the `Storer` interface for unit testing
- **Generated**: Uses testify/mock for automatic mock generation

#### Usage Example
```go
func TestHandleGetUser(t *testing.T) {
    mockStore := &MockStore{}
    testLogger := zap.NewNop()

    expectedUser := &domain.User{
        ID:    uuid.New(),
        Email: "test@example.com",
        Name:  stringPtr("Test User"),
    }

    mockStore.On("GetUserByID", mock.Anything, expectedUser.ID).
        Return(*expectedUser, nil)

    handler := HandleGetUser(mockStore, testLogger)

    // Test the handler...
}
```

## CI/CD Testing Pipeline

The project uses GitHub Actions for automated testing with a comprehensive pipeline.

### Pipeline Jobs

#### 1. Unit Tests (`unit-tests`)
- **Trigger**: Every push and pull request
- **Runtime**: ~2 seconds
- **Commands**:
  ```bash
  go test -v ./...
  go test -race -short ./...
  ```

#### 2. Integration Tests (`integration-tests`)
- **Trigger**: PRs to main/develop branches and pushes to main/develop
- **Runtime**: ~4 seconds
- **Features**:
  - PostgreSQL 15-alpine container via testcontainers
  - Full database migration testing
  - CRUD operation validation
  - Constraint violation testing

#### 3. Build (`build`)
- **Purpose**: Ensures code compiles successfully
- **Commands**:
  ```bash
  go build -v ./...
  go build -o bin/server ./cmd/server
  ```

#### 4. Lint (`lint`)
- **Purpose**: Code quality enforcement
- **Tool**: golangci-lint-action v4
- **Configuration**: Uses project `.golangci.yml`

### Pipeline Optimizations

#### Caching
- Go module cache: `~/go/pkg/mod`
- Key: Based on `go.sum` hash
- Fallback: OS-specific Go cache

#### Docker Integration
- Integration tests use Docker daemon for testcontainers
- PostgreSQL containers with health checks
- Automatic cleanup on test completion

## Test Coverage

### Coverage Goals
- **Target**: >80% code coverage
- **Current**: 29.4% overall coverage
- **Measurement**: `go test -cover` and CI reports
- **Focus Areas**:
  - API handlers (request/response logic)
  - Business logic (validation, processing)
  - Error handling (edge cases, failures)
  - Database operations (CRUD, constraints)

### Current Coverage Breakdown

**Overall Coverage: 29.4%**

#### By Package:
- `internal/api/health`: **100.0%** ✅
- `internal/api/user`: **100.0%** ✅
- `internal/api/account`: **86.2%** ✅
- `internal/api/common`: **69.1%** ✅
- `internal/api/calendar`: **55.8%** ⚠️
- `internal/api/gmail`: **53.1%** ⚠️
- `internal/api/rule`: **59.0%** ⚠️
- `internal/api/log`: **80.0%** ✅
- `internal/api/auth`: **25.0%** ❌
- `internal/crypto`: **83.9%** ✅
- `internal/database`: **71.0%** ✅
- `internal/logger`: **87.9%** ✅
- `internal/store`: **0.9%** ❌ (needs significant improvement)
- `internal/worker`: **2.3%** ❌ (needs significant improvement)

#### Key Findings:
- **API Layer**: Good coverage (36.8% average), with health/user endpoints fully covered
- **Store Layer**: Very low coverage (0.9%), critical business logic largely untested
- **Worker Layer**: Minimal coverage (2.3%), background processing logic needs testing
- **Infrastructure**: Good coverage for crypto, database, and logging utilities

### Coverage Report Generation
```bash
# Generate coverage report
go test -cover ./...

# Generate detailed coverage profile
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Coverage by package
go test -cover ./internal/...
```

## Testing Best Practices

### Test Structure
```go
func TestFunctionName(t *testing.T) {
    // Arrange
    setupTestData()
    mockDependencies()

    // Act
    result := callFunctionUnderTest()

    // Assert
    assertExpectedBehavior(result)
}
```

### Naming Conventions
- **Test files**: `*_test.go` alongside source files
- **Test functions**: `TestFunctionName` or `TestFunctionName_Scenario`
- **Build tags**: `//go:build integration` for integration tests

### Test Categories
- **Unit tests**: Test individual functions
- **Integration tests**: Test component interactions
- **End-to-end tests**: Full application workflows (future)

### Mock Usage
- Use mocks for external dependencies (database, APIs)
- Verify mock expectations with `mock.AssertExpectations(t)`
- Keep mocks simple and focused

### Error Testing
- Test both success and failure scenarios
- Verify error messages and status codes
- Test edge cases and boundary conditions

## Database Testing

### Migration Testing
- Tests verify schema creation and updates
- Validates table existence and constraints
- Ensures foreign key relationships

### CRUD Testing
- Create, Read, Update, Delete operations
- Constraint validation (unique, foreign keys)
- Transaction handling
- Error scenarios (not found, conflicts)

### Test Data Management
- Clean test data between tests
- Use transactions for isolation
- Avoid test data pollution

## API Testing

### Handler Testing
- HTTP request/response validation
- Status code verification
- JSON serialization/deserialization
- Header validation (Content-Type, CORS)

### Middleware Testing
- Authentication middleware
- CORS headers
- Request logging
- Error handling

### Integration Testing
- Full request lifecycle
- Database state changes
- External API mocking (Google APIs)

## Worker Testing

### Background Process Testing
- Worker initialization and startup
- Periodic execution verification
- Concurrent processing validation
- Error handling and recovery

### Automation Logic Testing
- Rule evaluation logic
- Action execution
- Deduplication mechanisms
- Logging verification

## Performance Testing

### Benchmarks
```go
func BenchmarkFunctionName(b *testing.B) {
    // Setup
    setupBenchmarkData()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Benchmark code
        functionUnderTest()
    }
}
```

### Race Detection
- CI runs `go test -race` to detect concurrency issues
- Manual race testing: `go test -race ./...`

## Debugging Tests

### Common Issues

#### Test Failures
```bash
# Run failing test with verbose output
go test -v -run TestFailingTest ./...

# Run with race detection
go test -race -run TestFailingTest ./...
```

#### Integration Test Issues
```bash
# Check Docker containers
docker ps

# View container logs
docker logs <container_id>

# Clean up containers
docker container prune
```

#### Mock Issues
```bash
# Verify mock expectations
mock.AssertExpectations(t)

// Debug mock calls
mockStore.AssertCalled(t, "MethodName", expectedArgs...)
```

## Test Maintenance

### Adding New Tests
1. Create `*_test.go` file alongside source
2. Follow naming conventions
3. Use appropriate test types (unit/integration)
4. Include edge cases and error scenarios
5. Update mocks if needed

### Updating Tests
- Keep tests in sync with code changes
- Update mocks when interfaces change
- Maintain test coverage goals
- Refactor tests for clarity

### Test Documentation
- Document complex test scenarios
- Explain mock setups and expectations
- Include comments for non-obvious assertions

## Continuous Integration

### GitHub Actions Workflow
- **File**: `.github/workflows/go.yml`
- **Triggers**: Push and PR to main/develop branches
- **Jobs**: unit-tests, integration-tests, build, lint
- **Go Version**: 1.24
- **PostgreSQL**: 15-alpine for integration tests

### Quality Gates
- All tests must pass ✅
- Code must compile without warnings ✅
- Linting must pass without errors ✅ (golangci-lint clean)
- Coverage thresholds met (future target: >80%)

### Current CI Status
- **Unit Tests**: ✅ Passing (29.4% coverage)
- **Integration Tests**: ✅ Passing (PostgreSQL 15-alpine containers)
- **Build**: ✅ Successful compilation
- **Lint**: ✅ No issues found
- **Race Detection**: ✅ Enabled in CI pipeline

### Automated Checks
- PR validation
- Branch protection rules
- Status checks for merges

## Future Testing Improvements

### Planned Enhancements
- **E2E Testing**: Full application testing with real Google APIs (sandboxed)
- **Load Testing**: Performance validation under load
- **Security Testing**: Automated security vulnerability scanning
- **Coverage Reporting**: Detailed coverage reports and trends
- **Mutation Testing**: Validate test effectiveness

### Coverage Improvement Priorities

#### High Priority (Critical Business Logic)
1. **Store Layer Testing**: `internal/store/` packages (currently 0.9% coverage)
   - Database CRUD operations
   - Token encryption/decryption
   - Business rule validation

2. **Worker Layer Testing**: `internal/worker/` packages (currently 2.3% coverage)
   - Background job processing
   - Google API integrations
   - Automation rule execution

#### Medium Priority (API Reliability)
3. **Authentication Testing**: `internal/api/auth` (currently 25.0% coverage)
   - OAuth flow validation
   - JWT token handling
   - CSRF protection

4. **Calendar API Testing**: `internal/api/calendar` (currently 55.8% coverage)
   - Event CRUD operations
   - Google Calendar API integration

#### Low Priority (Already Well Covered)
5. **Health/User APIs**: Already at 100% coverage ✅
6. **Infrastructure**: Crypto, database, logger well tested ✅

### Test Automation
- **Test Generation**: Automated test case generation
- **Property Testing**: Generate test cases from properties
- **Fuzz Testing**: Automated input generation and crash detection

## Test Execution Results

### Current Test Status
- **Unit Tests**: ✅ All passing
- **Integration Tests**: ✅ All passing (TestDatabaseIntegration: 4.83s)
- **Code Quality**: ✅ golangci-lint clean (no issues)
- **Build**: ✅ Successful compilation
- **Race Detection**: ✅ Enabled and passing

### Test Execution Times
- **Unit Tests**: ~0.1-0.4s per package
- **Integration Tests**: ~4.8s (with PostgreSQL container setup)
- **Code Quality**: ~5s timeout (actual: <1s)
- **Build**: ~2s

### Test Environment
- **Go Version**: 1.24.0
- **Test Framework**: testify/assert, testify/require
- **Mock Framework**: testify/mock
- **Integration**: testcontainers-go with PostgreSQL 15-alpine
- **CI**: GitHub Actions with Ubuntu latest

## Support

For testing issues:
1. Check test output and logs
2. Verify Docker is running for integration tests
3. Ensure all dependencies are installed
4. Review CI pipeline status
5. Check mock expectations and setup

## References

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Framework](https://github.com/stretchr/testify)
- [Testcontainers](https://golang.testcontainers.org/)
- [golangci-lint](https://golangci-lint.run/)
- [GitHub Actions](https://docs.github.com/en/actions)