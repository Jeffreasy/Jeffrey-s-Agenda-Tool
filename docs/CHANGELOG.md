# Changelog

All notable changes to the Agenda Automator Backend will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Dual-service automation platform** supporting both Google Calendar and Gmail
- **Automated calendar processing system** with 2-minute intervals
- **Comprehensive calendar coverage** for all events (1970-2100)
- **Flexible automation rules** with JSONB-based trigger conditions and actions
- **Full calendar CRUD operations** via REST API (create, read, update, delete events)
- **Aggregated calendar events** fetching from multiple accounts/calendars
- **Comprehensive automation logging** with success/failure/skipped status tracking
- **Complete Gmail integration** with message retrieval, sending, and management
- **Gmail automation rules** with advanced trigger conditions (sender, subject, label matching)
- **Gmail actions** including auto-reply, forwarding, label management, and status changes
- **Gmail History API integration** for efficient incremental message synchronization
- **Gmail drafts and labels management** via REST API
- **JWT-based authentication** with 7-day token expiration
- **Backend-driven OAuth 2.0 flow** with CSRF protection
- **Complete REST API** with 25+ endpoints for full application functionality
- **Parallel account processing** for multiple Google accounts (Calendar + Gmail)
- **Token refresh automation** with secure encrypted storage
- **User management** via Google OAuth integration
- **Connected accounts management** with status tracking and Gmail sync controls
- **Rule management** (CRUD operations for both Calendar and Gmail automation rules)
- **Calendar list endpoint** (`GET /accounts/{accountId}/calendars`) for retrieving all accessible calendars
- **Multi-calendar support** in frontend calendar view with fallback calendar IDs
- **Health check endpoint** for monitoring
- PostgreSQL database with comprehensive schema including Gmail tables
- Docker Compose setup for local development
- Google Calendar and Gmail API integrations
- Comprehensive documentation suite

### Changed
- **Worker frequency**: Changed from 5-minute to 2-minute intervals for regular processing
- **Monitoring scope**: Processes all historical and future events (1970-2100) instead of limited windows
- **Database design**: Enhanced with JSONB fields for flexible rule configuration
- **OAuth flow**: Completely refactored from frontend-driven to secure backend-driven implementation
- **API architecture**: Expanded from basic endpoints to comprehensive REST API with authentication
- **Worker logic**: Enhanced with deduplication, comprehensive logging, and flexible rule processing
- **Platform scope**: Extended from calendar-only to dual-service automation (Calendar + Gmail)
- **OAuth scopes**: Expanded to include comprehensive Gmail API permissions

### Performance
- **Parallel processing**: Multiple accounts processed simultaneously for both Calendar and Gmail
- **Efficient querying**: Optimized database operations with proper indexing
- **Token management**: Automated refresh prevents authentication failures
- **Comprehensive coverage**: Processes unlimited historical and future events
- **Gmail incremental sync**: History API integration reduces API calls and improves performance
- **Smart deduplication**: Prevents duplicate actions across both Calendar and Gmail automation
- **Database optimizations**: Added comprehensive indexing strategy including GIN indexes for JSON/arrays, functional indexes for calendar event deduplication, partial indexes for active records, and fill factor optimizations for frequently updated tables
- **Query performance**: Implemented specialized indexes for Gmail label searches, calendar event ID lookups, automation log filtering, and case-insensitive email searches
- **Data integrity**: Added check constraints and length limits to prevent invalid data and improve storage efficiency

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
- Go 1.24.0 compatibility
- PostgreSQL database with pgx driver
- Chi HTTP router with CORS middleware
- AES-256-GCM encryption for sensitive OAuth tokens
- Structured logging with zap and file rotation
- Google Calendar and Gmail API integrations
- Docker containerization
- golang-migrate for database migrations
- Comprehensive test coverage with testify

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