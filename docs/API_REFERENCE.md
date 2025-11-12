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

### Automation Rules

#### Create Automation Rule

Create a new automation rule for shift monitoring and reminder creation.

**Endpoint:** `POST /rules`

**Description:** Creates an automation rule that defines how the system should monitor calendar events and create reminders. The rule uses flexible JSONB configuration for trigger conditions and action parameters.

**Request Body:**
```json
{
  "connected_account_id": "uuid (required)",
  "name": "string (required)",
  "trigger_conditions": {
    "summary_equals": "Dienst"
  },
  "action_params": {
    "offset_minutes": -60,
    "duration_min": 5
  }
}
```

**Request Example:**
```bash
curl -X POST http://localhost:8080/api/v1/rules \
  -H "Content-Type: application/json" \
  -d '{
    "connected_account_id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Shift Reminders",
    "trigger_conditions": {
      "summary_equals": "Dienst"
    },
    "action_params": {
      "offset_minutes": -60,
      "duration_min": 5
    }
  }'
```

**Response (201 Created):**
```json
{
  "ID": "uuid",
  "ConnectedAccountID": "uuid",
  "Name": "Shift Reminders",
  "IsActive": true,
  "TriggerConditions": "base64-encoded-jsonb",
  "ActionParams": "base64-encoded-jsonb",
  "CreatedAt": "2025-11-12T14:00:00Z",
  "UpdatedAt": "2025-11-12T14:00:00Z"
}
```

**Trigger Conditions:**
- `summary_equals`: Exact match for event summary (e.g., "Dienst")
- `summary_contains`: Array of strings to match in summary (fallback)

**Action Parameters:**
- `offset_minutes`: Minutes before event to create reminder (negative = before)
- `duration_min`: Duration of reminder event in minutes
- `title_prefix`: Optional prefix for reminder titles

**Error Responses:**
- `400 Bad Request`: Invalid JSON or missing required fields
- `500 Internal Server Error`: Database error

**Notes:**
- Rules are automatically applied by the real-time worker
- The worker runs every 30 seconds and monitors 1 year ahead
- Reminders are created with smart titles: "{Vroeg/Laat} {A/R}"

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