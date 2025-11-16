# API Reference

This document provides comprehensive documentation for the Agenda Automator REST API.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

The API uses JWT (JSON Web Token) authentication for protected endpoints. Users authenticate via Google OAuth 2.0, which returns a JWT token that must be included in the `Authorization` header for subsequent requests.

**Authentication Header Format:**
```
Authorization: Bearer <jwt_token>
```

### Obtaining a JWT Token

1. Initiate Google OAuth flow: `GET /api/v1/auth/google/login`
2. Complete OAuth callback (handled automatically)
3. Receive JWT token in redirect URL: `${CLIENT_BASE_URL}/dashboard?token=<jwt_token>`

## Response Format

All responses are in JSON format. Successful responses include the requested data, while errors return an error object with a descriptive message.

### Success Response Structure
```json
{
  "data": "...",
  "message": "Optional success message"
}
```

### Error Response Structure
```json
{
  "error": "Error message description"
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
- Redirects to `${CLIENT_BASE_URL}/dashboard?token=<jwt_token>`

**Process:**
1. Validates state token from cookie
2. Exchanges authorization code for access/refresh tokens
3. Fetches user profile (email, name, Google ID)
4. Creates or updates user in database
5. Encrypts and stores OAuth tokens
6. Generates and returns JWT token

**Error Responses:**
- `400 Bad Request`: Invalid or missing state token
- `500 Internal Server Error`: Token exchange or user creation failed

---

### User Management

#### Get Current User

Retrieve information about the authenticated user.

**Endpoint:** `GET /api/v1/me` or `GET /api/v1/users/me`

**Authentication:** Required (JWT token)

**Description:** Returns the profile information of the currently authenticated user.

**Parameters:** None

**Response (200 OK):**
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "John Doe",
  "created_at": "2025-11-15T19:00:00Z",
  "updated_at": "2025-11-15T19:00:00Z"
}
```

**Error Responses:**
- `401 Unauthorized`: Missing or invalid JWT token

---

### Connected Accounts Management

#### Get Connected Accounts

Retrieve all Google Calendar accounts connected by the authenticated user.

**Endpoint:** `GET /api/v1/accounts`

**Authentication:** Required (JWT token)

**Description:** Returns a list of all connected Google accounts for the current user.

**Parameters:** None

**Response (200 OK):**
```json
[
  {
    "id": "uuid",
    "user_id": "uuid",
    "provider": "google",
    "email": "calendar@example.com",
    "provider_user_id": "google_user_id",
    "scopes": ["https://www.googleapis.com/auth/calendar.events"],
    "status": "active",
    "created_at": "2025-11-15T19:00:00Z",
    "updated_at": "2025-11-15T19:00:00Z",
    "last_checked": "2025-11-15T19:00:00Z"
  }
]
```

**Error Responses:**
- `401 Unauthorized`: Missing or invalid JWT token

---

#### Delete Connected Account

Remove a connected Google Calendar account.

**Endpoint:** `DELETE /api/v1/accounts/{accountId}`

**Authentication:** Required (JWT token)

**Description:** Deletes a connected account and all associated data (rules, logs).

**Path Parameters:**
- `accountId`: UUID of the connected account

**Response (204 No Content):** Empty response

**Error Responses:**
- `401 Unauthorized`: Missing or invalid JWT token
- `404 Not Found`: Account not found or doesn't belong to user

---

### Automation Rules Management

#### Create Automation Rule

Create a new automation rule for calendar event processing.

**Endpoint:** `POST /api/v1/accounts/{accountId}/rules`

**Authentication:** Required (JWT token)

**Description:** Creates an automation rule that defines how the system should monitor calendar events and execute actions.

**Path Parameters:**
- `accountId`: UUID of the connected account

**Request Body:**
```json
{
  "name": "Shift Reminders",
  "trigger_conditions": {
    "summary_equals": "Dienst",
    "summary_contains": ["shift", "work"],
    "location_contains": ["office", "remote"]
  },
  "action_params": {
    "offset_minutes": -60,
    "new_event_title": "Reminder: {summary}",
    "duration_min": 5
  }
}
```

**Trigger Conditions:**
- `summary_equals` (string): Exact match for event summary
- `summary_contains` (array): Event summary must contain any of these strings
- `location_contains` (array): Event location must contain any of these strings

**Action Parameters:**
- `offset_minutes` (number): Minutes before event to create reminder (negative = before)
- `new_event_title` (string): Title template for created events
- `duration_min` (number): Duration of reminder event in minutes

**Response (201 Created):**
```json
{
  "id": "uuid",
  "connected_account_id": "uuid",
  "name": "Shift Reminders",
  "is_active": true,
  "trigger_conditions": {...},
  "action_params": {...},
  "created_at": "2025-11-15T19:00:00Z",
  "updated_at": "2025-11-15T19:00:00Z"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid JSON or missing required fields
- `401 Unauthorized`: Missing or invalid JWT token
- `404 Not Found`: Account not found or doesn't belong to user

---

#### Get Automation Rules

Retrieve all automation rules for a connected account.

**Endpoint:** `GET /api/v1/accounts/{accountId}/rules`

**Authentication:** Required (JWT token)

**Description:** Returns all automation rules associated with the specified account.

**Path Parameters:**
- `accountId`: UUID of the connected account

**Response (200 OK):**
```json
[
  {
    "id": "uuid",
    "connected_account_id": "uuid",
    "name": "Shift Reminders",
    "is_active": true,
    "trigger_conditions": {...},
    "action_params": {...},
    "created_at": "2025-11-15T19:00:00Z",
    "updated_at": "2025-11-15T19:00:00Z"
  }
]
```

---

#### Update Automation Rule

Modify an existing automation rule.

**Endpoint:** `PUT /api/v1/rules/{ruleId}`

**Authentication:** Required (JWT token)

**Description:** Updates the name, trigger conditions, and action parameters of an automation rule.

**Path Parameters:**
- `ruleId`: UUID of the automation rule

**Request Body:** Same as create rule

**Response (200 OK):** Updated rule object

---

#### Toggle Rule Status

Enable or disable an automation rule.

**Endpoint:** `PUT /api/v1/rules/{ruleId}/toggle`

**Authentication:** Required (JWT token)

**Description:** Toggles the active status of an automation rule.

**Path Parameters:**
- `ruleId`: UUID of the automation rule

**Response (200 OK):** Updated rule object with toggled `is_active` field

---

#### Delete Automation Rule

Remove an automation rule.

**Endpoint:** `DELETE /api/v1/rules/{ruleId}`

**Authentication:** Required (JWT token)

**Description:** Deletes an automation rule.

**Path Parameters:**
- `ruleId`: UUID of the automation rule

**Response (204 No Content):** Empty response

---

### Automation Logs

#### Get Automation Logs

Retrieve execution logs for automation rules.

**Endpoint:** `GET /api/v1/accounts/{accountId}/logs`

**Authentication:** Required (JWT token)

**Description:** Returns the most recent automation execution logs for the specified account (limited to 50 entries).

**Path Parameters:**
- `accountId`: UUID of the connected account

**Response (200 OK):**
```json
[
  {
    "id": 123,
    "connected_account_id": "uuid",
    "rule_id": "uuid",
    "timestamp": "2025-11-15T19:00:00Z",
    "status": "success",
    "trigger_details": {
      "google_event_id": "event_id",
      "trigger_summary": "Dienst",
      "trigger_time": "2025-11-15T08:00:00Z"
    },
    "action_details": {
      "created_event_id": "reminder_event_id",
      "created_event_summary": "Reminder: Dienst",
      "reminder_time": "2025-11-15T07:00:00Z"
    },
    "error_message": ""
  }
]
```

**Status Values:**
- `success`: Rule executed successfully
- `failure`: Rule execution failed
- `skipped`: Rule was skipped (duplicate or other condition)

---

### Calendar Events Management

#### List Calendars

Retrieve all calendars accessible by a connected Google Calendar account.

**Endpoint:** `GET /api/v1/accounts/{accountId}/calendars`

**Authentication:** Required (JWT token)

**Description:** Returns a list of all calendars that the connected Google account has access to, including primary calendar, secondary calendars, and shared calendars.

**Path Parameters:**
- `accountId`: UUID of the connected account

**Response (200 OK):**
```json
[
  {
    "id": "primary",
    "summary": "My Calendar",
    "description": "Primary calendar",
    "primary": true,
    "accessRole": "owner"
  },
  {
    "id": "work@group.calendar.google.com",
    "summary": "Work Calendar",
    "description": "Shared work calendar",
    "primary": false,
    "accessRole": "writer"
  }
]
```

**Error Responses:**
- `401 Unauthorized`: Missing or invalid JWT token
- `404 Not Found`: Account not found

---

#### Get Calendar Events

Retrieve calendar events from a connected Google Calendar account.

**Endpoint:** `GET /api/v1/accounts/{accountId}/calendar/events`

**Authentication:** Required (JWT token)

**Description:** Fetches calendar events from the specified account's primary calendar or a specific calendar.

**Path Parameters:**
- `accountId`: UUID of the connected account

**Query Parameters:**
- `calendarId` (optional): Calendar ID (defaults to "primary")
- `timeMin` (optional): Start time in RFC3339 format (defaults to current time)
- `timeMax` (optional): End time in RFC3339 format (defaults to 3 months ahead)

**Response (200 OK):**
```json
[
  {
    "id": "event_id",
    "summary": "Meeting",
    "description": "Team meeting",
    "start": {
      "dateTime": "2025-11-15T10:00:00Z"
    },
    "end": {
      "dateTime": "2025-11-15T11:00:00Z"
    },
    "location": "Conference Room A"
  }
]
```

---

#### Create Calendar Event

Create a new event in a connected Google Calendar.

**Endpoint:** `POST /api/v1/accounts/{accountId}/calendar/events`

**Authentication:** Required (JWT token)

**Description:** Creates a new calendar event using the Google Calendar API format.

**Path Parameters:**
- `accountId`: UUID of the connected account

**Query Parameters:**
- `calendarId` (optional): Calendar ID (defaults to "primary")

**Request Body:** Google Calendar Event object
```json
{
  "summary": "New Event",
  "description": "Event description",
  "start": {
    "dateTime": "2025-11-15T10:00:00Z",
    "timeZone": "Europe/Amsterdam"
  },
  "end": {
    "dateTime": "2025-11-15T11:00:00Z",
    "timeZone": "Europe/Amsterdam"
  },
  "location": "Office"
}
```

**Response (201 Created):** Created Google Calendar Event object

---

#### Update Calendar Event

Modify an existing calendar event.

**Endpoint:** `PUT /api/v1/accounts/{accountId}/calendar/events/{eventId}`

**Authentication:** Required (JWT token)

**Description:** Updates an existing calendar event.

**Path Parameters:**
- `accountId`: UUID of the connected account
- `eventId`: Google Calendar event ID

**Query Parameters:**
- `calendarId` (optional): Calendar ID (defaults to "primary")

**Request Body:** Updated Google Calendar Event object

**Response (200 OK):** Updated Google Calendar Event object

---

#### Delete Calendar Event

Remove a calendar event.

**Endpoint:** `DELETE /api/v1/accounts/{accountId}/calendar/events/{eventId}`

**Authentication:** Required (JWT token)

**Description:** Deletes a calendar event.

**Path Parameters:**
- `accountId`: UUID of the connected account
- `eventId`: Google Calendar event ID

**Query Parameters:**
- `calendarId` (optional): Calendar ID (defaults to "primary")

**Response (204 No Content):** Empty response

---

### Aggregated Events

#### Get Aggregated Events

Retrieve events from multiple connected accounts and calendars.

**Endpoint:** `POST /api/v1/calendar/aggregated-events`

**Authentication:** Required (JWT token)

**Description:** Fetches and combines calendar events from multiple accounts and calendars.

**Request Body:**
```json
{
  "accounts": [
    {
      "accountId": "uuid",
      "calendarId": "primary"
    },
    {
      "accountId": "uuid",
      "calendarId": "work@group.calendar.google.com"
    }
  ]
}
```

**Response (200 OK):** Array of Google Calendar Event objects from all specified accounts/calendars

---

### Health Check

#### API Health Check

Check if the API server is running and healthy.

**Endpoint:** `GET /api/v1/health`

**Authentication:** None required

**Description:** Performs a basic health check of the API server.

**Parameters:** None

**Response (200 OK):**
```json
{
  "status": "ok"
}
```

## Rate Limiting

Currently, no rate limiting is implemented. Consider adding rate limiting in production to prevent abuse.

## CORS

The API includes CORS middleware allowing requests from configured origins (specified in `ALLOWED_ORIGINS` environment variable).

## Content Types

- Request: `application/json`
- Response: `application/json`

## HTTP Status Codes

- `200 OK`: Successful GET requests
- `201 Created`: Successful POST requests creating resources
- `204 No Content`: Successful DELETE requests
- `307 Temporary Redirect`: OAuth login redirect
- `303 See Other`: OAuth callback redirect
- `400 Bad Request`: Invalid request data or parameters
- `401 Unauthorized`: Missing or invalid authentication
- `403 Forbidden`: Access denied to resource
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

## SDKs and Libraries

No official SDKs are available yet. Use standard HTTP clients or libraries like:
- JavaScript: `fetch()` or `axios`
- Python: `requests`
- Go: `net/http`

## Versioning

The API uses URL versioning (`/api/v1/`). Future versions will use `/api/v2/`, etc.

## Error Handling

All errors return a JSON object with an `error` field containing a descriptive message. Check the HTTP status code for the error type.

## Data Types

- **UUID**: Universally unique identifier (string format)
- **Timestamp**: ISO 8601 format (e.g., "2025-11-15T19:00:00Z")
- **JSONB**: PostgreSQL JSONB fields for flexible configuration objects