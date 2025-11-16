# Setup Guide

This guide provides detailed instructions for setting up the Agenda Automator backend for development and testing.

## Prerequisites

### System Requirements

- **Operating System:** Windows 10/11, macOS, or Linux
- **RAM:** Minimum 4GB, recommended 8GB
- **Disk Space:** 2GB free space
- **Network:** Internet connection for downloading dependencies

### Required Software

#### Go Programming Language
- **Version:** 1.24.0 or higher
- **Download:** https://go.dev/doc/install
- **Verification:**
  ```bash
  go version
  # Should output: go version go1.24.x or higher
  ```

#### Docker Desktop
- **Download:** https://www.docker.com/products/docker-desktop
- **Purpose:** Runs PostgreSQL container
- **Verification:**
  ```bash
  docker --version
  docker compose version
  ```


#### Git (Optional)
- **Download:** https://git-scm.com/downloads
- **Purpose:** Version control and cloning repositories

## Project Setup

### 1. Clone or Download the Project

If using Git:
```bash
git clone <repository-url>
cd AgendaTool/Jeffrey-s-Agenda-Tool BACKEND
```

Or download and extract the ZIP file to your preferred location.

### 2. Environment Configuration

Create a `.env` file in the project root directory:

```bash
# Create .env file
touch .env
```

Copy the following configuration into `.env` and modify the values:

```env
#---------------------------------------------------
# 1. APPLICATION CONFIGURATION
#---------------------------------------------------
APP_ENV=development
API_PORT=8080

#---------------------------------------------------
# 2. DATABASE (POSTGRES)
#---------------------------------------------------
# Note: We use port 5433 to avoid conflicts
POSTGRES_USER=postgres
POSTGRES_PASSWORD=Bootje12
POSTGRES_DB=agenda_automator
POSTGRES_HOST=localhost
POSTGRES_PORT=5433

# Full URL for the Go application
DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"


#---------------------------------------------------
# 4. SECURITY & ENCRYPTION
#---------------------------------------------------
# Must be exactly 32 characters (AES-256)
ENCRYPTION_KEY="IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea"

# JWT secret key (32+ characters recommended)
JWT_SECRET_KEY="YOUR_SECURE_JWT_SECRET_KEY_32_CHARS_MIN"

#---------------------------------------------------
# 5. OAUTH CLIENTS (Google)
#---------------------------------------------------
# Obtained from Google Cloud Console
CLIENT_BASE_URL="http://localhost:3000"
OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/google/callback"

GOOGLE_OAUTH_CLIENT_ID="YOUR-CLIENT-ID-HERE.apps.googleusercontent.com"
GOOGLE_OAUTH_CLIENT_SECRET="YOUR-CLIENT-SECRET-HERE"

#---------------------------------------------------
# 6. CORS CONFIGURATION
#---------------------------------------------------
# Allowed origins for CORS (comma-separated)
ALLOWED_ORIGINS="http://localhost:3000,http://localhost:3001"
```

**Important Notes:**
- Change `POSTGRES_PASSWORD` to a secure password
- The `ENCRYPTION_KEY` must be exactly 32 characters
- The `JWT_SECRET_KEY` should be at least 32 characters for security
- Replace Google OAuth credentials with real values from Google Cloud Console
- Ensure all required Google APIs are enabled (Calendar, Gmail, People)

### 3. Google OAuth Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing one
3. Enable the following APIs:
   - Google Calendar API
   - Gmail API
   - Google People API (optional, for contacts)
4. Create OAuth 2.0 credentials
5. Add authorized redirect URIs:
    - `http://localhost:8080/api/v1/auth/google/callback`
6. Copy Client ID and Client Secret to `.env`

**Important:** The redirect URI must exactly match the `OAUTH_REDIRECT_URL` in your `.env` file. The backend handles the complete OAuth flow, so the redirect goes to the backend, not the frontend.

**Required OAuth Scopes:** The application requires these Google API scopes:
- `https://www.googleapis.com/auth/calendar` (Calendar access)
- `https://www.googleapis.com/auth/calendar.events` (Calendar events)
- `https://www.googleapis.com/auth/gmail.modify` (Full Gmail access)
- `https://www.googleapis.com/auth/gmail.compose` (Create drafts)
- `https://www.googleapis.com/auth/gmail.labels` (Manage labels)
- `https://www.googleapis.com/auth/userinfo.email` (User email)
- `https://www.googleapis.com/auth/userinfo.profile` (User profile)

## Database Setup

### Start Database Container

```bash
# Start PostgreSQL in background
docker compose up -d
```

**Verification:**
```bash
# Check if container is running
docker ps

# Should show postgres container
```

### Run Database Migrations

```bash
# Run migrations (adjust password if changed)
migrate -database "postgres://postgres:Bootje12@localhost:5433/agenda_automator?sslmode=disable" -path db/migrations up
```

**Expected Output:**
```
1/u up (000001)
```

**Troubleshooting:**
- If migration fails, ensure PostgreSQL container is running
- Check database connection with:
  ```bash
  psql "postgres://postgres:Bootje12@localhost:5433/agenda_automator?sslmode=disable" -c "SELECT version();"
  ```

## Go Dependencies

### Install Dependencies

```bash
# Download all Go modules
go mod tidy

# Verify dependencies
go mod verify
```

**Expected Output:**
```
go: downloading github.com/go-chi/chi/v5 v5.0.10
go: downloading github.com/jackc/pgx/v5 v5.5.4
...
```

## Running the Application

### Development Mode

```bash
# Run the application
go run ./cmd/server
```

**Expected Output:**
```
2025/11/16 15:00:00 Successfully connected to database.
2025/11/16 15:00:00 Starting worker...
2025/11/16 15:00:00 Application starting API server on port 8080...
2025/11/16 15:00:00 [Worker] Running work cycle...
2025/11/16 15:00:00 [Worker] Found 0 active accounts to check.
```

### Automation Processing Features

The application includes **dual-service automation processing** for both Google Calendar and Gmail with the following capabilities:

- **Processing Frequency:** Every 2 minutes (regular batch processing)
- **Coverage Window:** All events from 1970-2100 (comprehensive historical and future processing)
- **Dual-Service Support:** Processes both Calendar events and Gmail messages
- **Flexible Automation Rules:** User-configurable JSONB-based trigger conditions and actions for both services
- **Smart Processing:**
  - **Calendar Triggers:** Summary text matching, location filtering, time-based conditions
  - **Gmail Triggers:** Sender matching, subject matching, label-based triggers
  - **Action Execution:** Configurable reminder creation, email replies, label management
  - **Deduplication:** Prevents duplicate actions via comprehensive logging
- **Calendar Automation:** Automatic event creation based on rule configurations
- **Gmail Automation:** Auto-replies, forwarding, labeling, and status management
- **Parallel Processing:** Handles multiple Google accounts simultaneously for both services
- **Incremental Sync:** Gmail History API integration for efficient message processing

### Verify Installation

#### API Health Check
```bash
curl http://localhost:8080/api/v1/health
```

**Expected Response:**
```json
{"status":"ok"}
```

#### Create a Test User
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "name": "Test User"}'
```

## Development Workflow

### Code Changes
- Make changes to Go files
- Restart the application (Ctrl+C then `go run cmd/server/main.go`)
- Test API endpoints

### Database Changes
- Modify migration files in `db/migrations/`
- Run migrations: `migrate -database "..." up`
- For rollbacks: `migrate -database "..." down 1`

### Environment Changes
- Update `.env` file
- Restart application to pick up changes

## Troubleshooting

### Common Issues

#### Port Already in Use
```
listen tcp :8080: bind: address already in use
```
**Solution:** Change `API_PORT` in `.env` or kill the process using the port.

#### Database Connection Failed
```
failed to connect to database
```
**Solutions:**
- Ensure Docker containers are running: `docker ps`
- Check PostgreSQL logs: `docker logs agenda-automator-postgres`
- Verify connection string in `.env`

#### Migration Errors
```
no change
```
**Solution:** Migration already applied. Check status with:
```bash
migrate -database "..." version
```

#### Go Module Issues
```
go mod tidy: error loading module
```
**Solutions:**
- Clear module cache: `go clean -modcache`
- Delete `go.sum` and run `go mod tidy`

### Logs and Debugging

#### Application Logs
The application outputs structured logs to stdout. Look for:
- Database connection status
- Worker cycle messages
- API request/response logs

#### Database Logs
```bash
docker logs agenda-automator-postgres
```

#### Container Management
```bash
# Stop containers
docker compose down

# Restart containers
docker compose restart

# View container status
docker compose ps
```

## Testing

### Unit Tests
```bash
go test ./...
```

### Code Quality Checks
```bash
# Run linting and code quality checks
golangci-lint run
```

### Integration Tests
```bash
# With database running
go test -tags=integration ./...
```

### API Testing
Use tools like:
- Postman
- curl
- HTTPie

## Next Steps

After successful setup:
1. Read the [API Reference](API_REFERENCE.md)
2. Review the [Architecture](ARCHITECTURE.md)
3. Set up the frontend application
4. Configure production deployment

## Support

For issues not covered here:
1. Check existing GitHub issues
2. Review application logs
3. Verify environment configuration
4. Test with minimal setup