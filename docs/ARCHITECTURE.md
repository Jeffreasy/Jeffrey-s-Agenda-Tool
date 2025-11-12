# Architecture Overview

This document provides a detailed overview of the Agenda Automator backend architecture, including components, data flow, and design decisions.

## System Overview

The Agenda Automator backend is a Go-based microservice that provides REST API endpoints for user management and OAuth account connections, while running a background worker to automate calendar-related tasks.

## Core Components

### 1. API Server (`/internal/api`)

The API server handles all reactive HTTP requests using the Chi router framework.

**Responsibilities:**
- User registration and management
- OAuth account connection storage
- Health check endpoints
- Request validation and response formatting

**Key Files:**
- `server.go`: Chi router setup and middleware configuration
- `handlers.go`: HTTP request handlers
- `json.go`: JSON encoding/decoding utilities

**Configuration:**
- Runs on configurable port (default: 8080)
- Uses structured logging
- Implements CORS for frontend integration

### 2. Background Worker (`/internal/worker`)

The worker is a high-frequency real-time monitoring system that proactively scans connected Google Calendar accounts and executes automation rules for shift management.

**Responsibilities:**
- **Real-time account scanning** (every 30 seconds)
- OAuth token refresh for expired tokens
- **Comprehensive calendar monitoring** (1 year ahead)
- **Intelligent shift detection** and reminder creation
- **Parallel processing** of multiple accounts

**Key Files:**
- `worker.go`: Main worker logic and scheduling

**Real-Time Monitoring Features:**
- **Frequency:** Every 30 seconds for immediate detection
- **Coverage:** Monitors events for the entire next year
- **Smart Detection:**
  - Recognizes "Dienst" events automatically
  - Classifies shifts: "Vroeg" (< 12:00) vs "Laat" (≥ 12:00)
  - Identifies teams: "A" (locations with "aa"/"appartementen") vs "R" (others)
- **Automated Actions:**
  - Creates reminders 1 hour before shifts
  - Generates smart titles: "{Vroeg/Laat} {A/R}"
  - Sets appropriate duration (5 minutes default)

**Design Pattern:**
- Uses high-frequency ticker for real-time responsiveness
- Runs concurrently with the API server
- Implements parallel goroutines for multi-account processing
- Graceful error handling and recovery

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

### User Registration Flow
1. Frontend sends POST to `/api/v1/users`
2. API validates request
3. Store creates user in database
4. API returns user data

### OAuth Account Connection Flow
1. User clicks "Login with Google" button on frontend
2. Frontend redirects to `/api/v1/auth/google/login`
3. Backend generates CSRF state token and redirects to Google OAuth consent page
4. User grants permissions on Google
5. Google redirects to `/api/v1/auth/google/callback` with authorization code
6. Backend validates state token and exchanges code for access/refresh tokens
7. Backend fetches user profile information from Google
8. Backend creates or updates user in database
9. Backend encrypts and stores tokens in connected_accounts table
10. Backend redirects user to frontend dashboard with success indicator

### Real-Time Automation Monitoring Flow
1. Worker runs **every 30 seconds** (real-time monitoring)
2. Queries **all active connected accounts** (no time filtering)
3. For each account:
   - Checks token expiry and refreshes if needed
   - Fetches calendar events for **entire next year**
   - Retrieves automation rules from database
   - **Automatically detects "Dienst" events**
   - **Classifies shifts**: Vroeg/Laat based on time, A/R based on location
   - **Creates reminder events** 1 hour before shifts with smart titles
   - Updates `last_checked` timestamp for performance tracking

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

- **Web Framework:** go-chi/chi for HTTP routing
- **Database:** jackc/pgx for PostgreSQL connectivity
- **Encryption:** Go's crypto/aes and crypto/cipher
- **Configuration:** godotenv for environment variables
- **Logging:** Built-in log package with structured output

## Deployment Architecture

For production deployment, consider:
- Containerization with Docker
- Orchestration with Kubernetes
- Load balancing for API servers
- Database replication for high availability