# Architecture Overview

This document provides a detailed overview of the Agenda Automator backend architecture, including components, data flow, and design decisions.

## System Overview

The Agenda Automator backend is a Go-based microservice that provides REST API endpoints for user management and OAuth account connections, while running a background worker to automate calendar-related tasks.

## Core Components

### 1. API Server (`/internal/api`)

The API server handles all reactive HTTP requests using the Chi router framework and provides a comprehensive REST API for calendar automation management.

**Responsibilities:**
- JWT-based authentication and user management
- Google OAuth 2.0 integration with CSRF protection
- Connected account management (Google Calendar accounts)
- Automation rule CRUD operations
- Calendar list retrieval for multi-calendar support
- Calendar event management (full CRUD operations)
- Aggregated calendar event fetching from multiple calendars
- Automation execution logs
- Request validation and response formatting

**Key Files:**
- `server.go`: Chi router setup, middleware configuration, and JWT authentication
- `handlers.go`: HTTP request handlers for all API endpoints
- `json.go`: JSON encoding/decoding utilities

**Configuration:**
- Runs on configurable port (default: 8080)
- Uses structured logging
- Implements CORS for frontend integration
- JWT tokens with 7-day expiration

### 2. Background Worker (`/internal/worker`)

The worker is an automated processing system that periodically scans connected Google Calendar accounts and executes user-defined automation rules.

**Responsibilities:**
- **Periodic account scanning** (every 2 minutes)
- OAuth token refresh for expired tokens
- **Comprehensive calendar event processing** (all events from 1970-2100)
- **Flexible automation rule execution** based on JSONB configurations
- **Parallel processing** of multiple accounts
- **Duplicate prevention** and logging of all actions

**Key Files:**
- `worker.go`: Main worker logic, scheduling, and automation execution

**Automation Features:**
- **Frequency:** Every 2 minutes for regular processing
- **Coverage:** Processes all historical and future events (1970-2100)
- **Flexible Rules:**
  - JSONB-based trigger conditions (summary matching, location filtering)
  - Configurable action parameters (timing, titles, duration)
  - Per-rule enable/disable toggles
- **Smart Processing:**
  - Prevents duplicate actions via log-based deduplication
  - Handles token refresh automatically
  - Processes multiple accounts concurrently
- **Comprehensive Logging:**
  - Success, failure, and skipped action logs
  - Detailed trigger and action metadata
  - Performance tracking with last_checked timestamps

**Design Pattern:**
- Uses ticker-based scheduling for consistent intervals
- Runs concurrently with the API server
- Implements parallel goroutines for multi-account processing
- Graceful error handling and recovery
- Database-driven deduplication to prevent spam

### 3. Database Layer (`/internal/store`)

The store implements the Repository pattern, providing a clean interface for database operations.

**Responsibilities:**
- CRUD operations for all entities
- Transaction management
- Query optimization

**Key Files:**
- `store.go`: Repository implementations

**Database:**
- PostgreSQL with pgx driver
- Connection pooling with pgxpool
- Schema migrations using golang-migrate

### 4. Domain Models (`/internal/domain`)

Contains Go structs representing database entities and business logic.

**Key Files:**
- `models.go`: Struct definitions and validation

### 5. Security & Encryption (`/internal/crypto`)

Handles sensitive data encryption using AES-GCM.

**Responsibilities:**
- OAuth token encryption/decryption
- Secure key management

**Key Files:**
- `crypto.go`: Encryption utilities

### 6. Database Connection (`/internal/database`)

Manages database connectivity and configuration.

**Key Files:**
- `database.go`: Connection setup

## Data Flow

### User Authentication Flow
1. User clicks "Login with Google" button on frontend
2. Frontend redirects to `GET /api/v1/auth/google/login`
3. Backend generates CSRF state token, stores in HTTP-only cookie, redirects to Google OAuth
4. User grants permissions on Google OAuth consent page
5. Google redirects to `GET /api/v1/auth/google/callback` with authorization code
6. Backend validates state token from cookie
7. Backend exchanges authorization code for access/refresh tokens
8. Backend fetches user profile from Google (email, name, Google ID)
9. Backend creates or updates user in database
10. Backend encrypts and stores OAuth tokens in connected_accounts table
11. Backend generates JWT token and redirects to `${CLIENT_BASE_URL}/dashboard?token=<jwt>`

### Automation Rule Execution Flow
1. Worker runs **every 2 minutes** (periodic processing)
2. Queries **all active connected accounts**
3. For each account:
    - Validates and refreshes OAuth tokens if needed
    - Fetches **all calendar events** (1970-2100, unlimited scope)
    - Retrieves active automation rules from database
    - For each event and rule combination:
        - Evaluates JSONB trigger conditions (summary, location matching)
        - Checks deduplication logs to prevent duplicate actions
        - If triggered: executes action (create reminder event)
        - Logs success/failure/skipped status with detailed metadata
    - Updates `last_checked` timestamp for account

## Security Considerations

- **Token Encryption:** All OAuth tokens are encrypted at rest using AES-256-GCM
- **Environment Variables:** Sensitive config stored in `.env` files
- **Input Validation:** All API inputs validated using struct tags
- **SQL Injection Prevention:** Parameterized queries throughout

## Scalability

- **Horizontal Scaling:** Stateless design allows multiple instances
- **Database Connection Pooling:** Efficient resource usage
- **Worker Isolation:** Background tasks don't block API responses

## Dependencies

- **Web Framework:** go-chi/chi/v5 for HTTP routing and middleware
- **Database:** jackc/pgx/v5 for PostgreSQL connectivity with connection pooling
- **Authentication:** golang-jwt/jwt/v5 for JWT token handling
- **OAuth:** golang.org/x/oauth2 for Google OAuth 2.0 integration
- **Google APIs:** google.golang.org/api/calendar/v3, google.golang.org/api/oauth2/v2
- **Encryption:** crypto/aes, crypto/cipher, crypto/rand for AES-GCM encryption
- **Configuration:** github.com/joho/godotenv for environment variables
- **UUID:** github.com/google/uuid for unique identifiers
- **Logging:** Built-in log package with structured output
- **Migrations:** golang-migrate for database schema management

## Deployment Architecture

For production deployment, consider:
- Containerization with Docker
- Orchestration with Kubernetes
- Load balancing for API servers
- Database replication for high availability