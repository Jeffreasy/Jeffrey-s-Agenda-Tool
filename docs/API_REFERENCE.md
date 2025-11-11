# API Reference

This document provides comprehensive documentation for the Agenda Automator REST API.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, the API does not implement authentication. All endpoints are publicly accessible. In production, consider implementing JWT or OAuth-based authentication.

## Response Format

All responses are in JSON format. Successful responses include the requested data, while errors return an error object.

### Success Response Structure
```json
{
  "status": "ok"
}
```

### Error Response Structure
```json
{
  "error": "Error message"
}
```

## Endpoints

### OAuth Authentication

#### Google Login

Initiate the Google OAuth 2.0 flow with CSRF protection.

**Endpoint:** `GET /api/v1/auth/google/login`

**Description:** Generates a secure state token, stores it in an HTTP-only cookie, and redirects the user to Google's OAuth consent page.

**Parameters:** None

**Response (307 Temporary Redirect):**
- Redirects to Google OAuth URL with state parameter

**Security Features:**
- CSRF protection via state token
- HTTP-only cookie for state storage
- Secure random state generation

---

#### Google OAuth Callback

Handle the OAuth callback from Google and complete the authentication flow.

**Endpoint:** `GET /api/v1/auth/google/callback`

**Description:** Validates the state token, exchanges the authorization code for tokens, fetches user profile information, creates/updates the user account, securely stores OAuth tokens, and redirects to the frontend.

**Query Parameters:**
- `code`: Authorization code from Google
- `state`: State token for CSRF protection

**Response (303 See Other):**
- Redirects to `${CLIENT_BASE_URL}/dashboard?success=true`

**Process:**
1. Validates state token from cookie
2. Exchanges authorization code for access/refresh tokens
3. Fetches user profile (email, name, Google ID)
4. Creates or updates user in database
5. Encrypts and stores OAuth tokens
6. Redirects to frontend dashboard

**Error Responses:**
- `400 Bad Request`: Invalid or missing state token
- `500 Internal Server Error`: Token exchange or user creation failed

---

### Health Check

Check if the API server is running and healthy.

**Endpoint:** `GET /api/v1/health`

**Description:** Performs a basic health check of the API server.

**Parameters:** None

**Response (200 OK):**
```json
{
  "status": "ok"
}
```

---

### User Management

#### Create User

Create a new user in the system.

**Endpoint:** `POST /users`

**Description:** Registers a new user with email and name.

**Request Body:**
```json
{
  "email": "string (required, valid email format)",
  "name": "string (required)"
}
```

**Request Example:**
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "name": "John Doe"
  }'
```

**Response (201 Created):**
```json
{
  "ID": "uuid",
  "Email": "john.doe@example.com",
  "Name": "John Doe",
  "CreatedAt": "2025-11-11T18:00:00Z",
  "UpdatedAt": "2025-11-11T18:00:00Z"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid email format or missing required fields
- `500 Internal Server Error`: Database error

---

### Connected Accounts (Legacy)

#### Create Connected Account

**⚠️ Legacy Endpoint:** This endpoint is primarily for development/testing purposes. In production, OAuth account connection is handled automatically through the `/api/v1/auth/google/callback` endpoint.

Link a Google OAuth account to an existing user manually.

**Endpoint:** `POST /users/{userID}/accounts`

**Description:** Manually stores OAuth tokens and account information for a Google account. This endpoint bypasses the standard OAuth flow and should only be used for testing.

**Parameters:**
- `userID` (path): UUID of the user

**Request Body:**
```json
{
  "provider": "google",
  "email": "string (required, valid email)",
  "provider_user_id": "string (required)",
  "access_token": "string (required)",
  "refresh_token": "string (optional)",
  "token_expiry": "string (required, RFC3339 timestamp)",
  "scopes": ["string"] (required, array of OAuth scopes)
}
```

**Request Example:**
```bash
curl -X POST http://localhost:8080/api/v1/users/123e4567-e89b-12d3-a456-426614174000/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "google",
    "email": "john.doe@gmail.com",
    "provider_user_id": "google-sub-id-12345",
    "access_token": "ya29.a0AfH6SMC...",
    "refresh_token": "1//0gM...",
    "token_expiry": "2025-11-11T19:00:00Z",
    "scopes": ["https://www.googleapis.com/auth/calendar.events"]
  }'
```

**Response (201 Created):**
```json
{
  "ID": "uuid",
  "UserID": "uuid",
  "Provider": "google",
  "Email": "john.doe@gmail.com",
  "ProviderUserID": "google-sub-id-12345",
  "AccessToken": "encrypted_data",
  "RefreshToken": "encrypted_data",
  "TokenExpiry": "2025-11-11T19:00:00Z",
  "Scopes": ["https://www.googleapis.com/auth/calendar.events"],
  "Status": "active",
  "CreatedAt": "2025-11-11T18:00:00Z",
  "UpdatedAt": "2025-11-11T18:00:00Z"
}
```

**Notes:**
- Only Google provider is currently supported
- Tokens are encrypted before storage using AES-GCM
- The encrypted tokens are returned as base64-encoded byte arrays

**Error Responses:**
- `400 Bad Request`: Invalid provider, missing required fields, or invalid UUID
- `404 Not Found`: User not found
- `500 Internal Server Error`: Database or encryption error

## Rate Limiting

Currently, no rate limiting is implemented. Consider adding rate limiting in production.

## CORS

The API includes CORS middleware allowing requests from configured origins (default: localhost:3000).

## Content Types

- Request: `application/json`
- Response: `application/json`

## HTTP Status Codes

- `200 OK`: Successful GET requests
- `201 Created`: Successful POST requests
- `400 Bad Request`: Invalid request data
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

## SDKs and Libraries

No official SDKs are available yet. Use standard HTTP clients or libraries like:
- JavaScript: `fetch()` or `axios`
- Python: `requests`
- Go: `net/http`

## Versioning

The API uses URL versioning (`/api/v1/`). Future versions will use `/api/v2/`, etc.

## Future Endpoints

The following endpoints are planned but not yet implemented:
- `GET /users/{userID}/accounts` - List connected accounts
- `PUT /users/{userID}/accounts/{accountID}` - Update account tokens
- `DELETE /users/{userID}/accounts/{accountID}` - Delete connected account
- `GET /users/{userID}` - Get user details
- Automation rule management endpoints