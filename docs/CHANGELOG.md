# Changelog

All notable changes to the Agenda Automator Backend will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Automated calendar processing system** with 2-minute intervals
- **Comprehensive calendar coverage** for all events (1970-2100)
- **Flexible automation rules** with JSONB-based trigger conditions and actions
- **Full calendar CRUD operations** via REST API (create, read, update, delete events)
- **Aggregated calendar events** fetching from multiple accounts/calendars
- **Comprehensive automation logging** with success/failure/skipped status tracking
- **JWT-based authentication** with 7-day token expiration
- **Backend-driven OAuth 2.0 flow** with CSRF protection
- **Complete REST API** with 16+ endpoints for full application functionality
- **Parallel account processing** for multiple Google Calendar accounts
- **Token refresh automation** with secure encrypted storage
- **User management** via Google OAuth integration
- **Connected accounts management** with status tracking
- **Rule management** (CRUD operations for automation rules)
- **Calendar list endpoint** (`GET /accounts/{accountId}/calendars`) for retrieving all accessible calendars
- **Multi-calendar support** in frontend calendar view with fallback calendar IDs
- **Health check endpoint** for monitoring
- PostgreSQL database with comprehensive schema
- Docker Compose setup for local development
- Google Calendar API integration
- Comprehensive documentation suite

### Changed
- **Worker frequency**: Changed from 5-minute to 2-minute intervals for regular processing
- **Monitoring scope**: Processes all historical and future events (1970-2100) instead of limited windows
- **Database design**: Enhanced with JSONB fields for flexible rule configuration
- **OAuth flow**: Completely refactored from frontend-driven to secure backend-driven implementation
- **API architecture**: Expanded from basic endpoints to comprehensive REST API with authentication
- **Worker logic**: Enhanced with deduplication, comprehensive logging, and flexible rule processing

### Performance
- **Parallel processing**: Multiple accounts processed simultaneously
- **Efficient querying**: Optimized database operations with proper indexing
- **Token management**: Automated refresh prevents authentication failures
- **Comprehensive coverage**: Processes unlimited historical and future events

### Security
- **JWT authentication**: Secure token-based API access
- **CSRF protection**: State tokens for OAuth flow security
- **Encrypted storage**: AES-GCM encryption for all sensitive OAuth tokens
- **Backend OAuth handling**: Secure server-side token exchange

## [1.0.0] - 2025-11-11

### Added
- Complete backend implementation for Agenda Automator
- API endpoints for user management and OAuth account connections
- Background worker with token refresh capabilities
- Database schema with proper indexing and constraints
- Environment-based configuration
- Structured logging
- Health check endpoint
- Comprehensive error handling

### Technical Details
- Go 1.21+ compatibility
- PostgreSQL database with pgx driver
- Chi HTTP router
- AES-256-GCM encryption for sensitive data
- Docker containerization
- golang-migrate for database migrations

---

## Types of Changes

- `Added` for new features
- `Changed` for changes in existing functionality
- `Deprecated` for soon-to-be removed features
- `Removed` for now removed features
- `Fixed` for any bug fixes
- `Security` in case of vulnerabilities

## Version Format

This project uses [Semantic Versioning](https://semver.org/):

- **MAJOR.MINOR.PATCH** (e.g., 1.2.3)
- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

## Contributing to Changelog

When contributing changes:

1. Add entries to the `[Unreleased]` section
2. Use present tense for descriptions
3. Group changes by type (Added, Changed, etc.)
4. Reference issue numbers when applicable
5. Move changes to a version section when releasing

Example:
```
### Added
- Add new API endpoint for user preferences (#123)

### Fixed
- Fix token refresh race condition (#124)
```

## Release Process

1. Update version in relevant files
2. Move unreleased changes to new version section
3. Commit with message: `chore: release v1.2.3`
4. Create git tag: `git tag v1.2.3`
5. Push tag: `git push origin v1.2.3`