# Changelog

All notable changes to the Agenda Automator Backend will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Real-time shift monitoring system** with 30-second intervals
- **Comprehensive calendar coverage** for entire next year
- **Intelligent shift classification**: Automatic "Vroeg/Laat" and "A/R" team detection
- **Automated reminder creation** with smart titles ("Vroeg A", "Laat R", etc.)
- **Parallel account processing** for multiple Google Calendar accounts
- **Flexible automation rules** with JSONB configuration
- **New API endpoint**: `POST /api/v1/rules` for automation rule management
- Backend-driven OAuth 2.0 flow with CSRF protection
- New OAuth endpoints: `/api/v1/auth/google/login` and `/api/v1/auth/google/callback`
- Automatic user creation/update during OAuth flow
- Secure token storage with AES-GCM encryption
- Google user profile fetching and account linking
- Initial project setup and structure
- REST API with Chi router
- PostgreSQL database integration with migrations
- Background worker for automation tasks
- Docker Compose setup for local development
- Google Calendar API integration
- User and connected accounts management
- Comprehensive documentation

### Changed
- **Worker frequency**: Upgraded from 5-minute to 30-second real-time monitoring
- **Monitoring window**: Expanded from 24 hours to 1 year comprehensive coverage
- **Database queries**: Removed `last_checked` filtering for continuous monitoring
- OAuth flow completely refactored from frontend-driven to backend-driven
- OAuth configuration centralized in main.go and injected into components
- Updated all documentation to reflect new OAuth architecture
- Legacy connected accounts endpoint marked as deprecated

### Performance
- **30x faster detection**: Changes detected within 30 seconds instead of 5 minutes
- **Parallel processing**: Multiple accounts processed simultaneously
- **Comprehensive coverage**: 1-year monitoring window vs previous limited scope

### Security
- Implemented CSRF protection for OAuth flow
- Backend now handles all OAuth token exchanges securely
- Enhanced security by removing token handling from frontend

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