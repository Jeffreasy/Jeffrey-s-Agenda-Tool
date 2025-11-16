# Agenda Automator - Backend

Dit is de Go-backend voor de Agenda Automator. Het biedt een REST API voor het beheren van gebruikers en gekoppelde Google accounts en een achtergrond-worker die agenda's monitort.

## ✨ Features

   * **REST API:** Een `chi`-gebaseerde API voor het beheren van `users` en `connected_accounts`.
   * **Backend-Driven OAuth:** Veilige OAuth 2.0 flow volledig afgehandeld door de backend met CSRF bescherming.
   * **Achtergrond Worker:** Een *long-running* goroutine die periodiek accounts controleert en tokens ververst.
   * **Veilig Tokenbeheer:** OAuth `access_token` en `refresh_token` worden versleuteld (AES-GCM) opgeslagen in de database.
   * **Automatisch Token Verversen:** De worker ververst automatisch verlopen Google OAuth-tokens.
   * **Google Calendar Automation:** De worker past automatiseringsregels toe op agenda-events en creëert herinneringen.
   * **Database:** PostgreSQL-database met een robuust, gemigreerd schema.
   * **Containerized:** Inclusief `docker-compose.yml` voor het lokaal opzetten van Postgres.

## 🏗️ Architectuur

De backend bestaat uit twee kerncomponenten die tegelijkertijd draaien:

1.  **De API-Server (`/internal/api`):**

       * Verantwoordelijk voor alle *reactieve* HTTP-verzoeken.
       * Handelt backend-driven OAuth 2.0 flow met Google af, inclusief CSRF bescherming.
       * Beheert gebruikersregistratie en het veilig opslaan van OAuth-tokens.
       * Draait op poort `8080` (instelbaar via `.env`).

2.  **De Worker (`/internal/worker`):**

       * Een *proactief* achtergrondproces (goroutine) dat op een timer draait (elke 2 minuten).
       * Leest `connected_accounts` uit de database.
       * Controleert of tokens verlopen zijn en ververst ze (via Google OAuth).
       * Haalt agenda-items op en past automatiseringsregels toe om herinneringen te creëren.

Deze twee componenten communiceren *nooit* direct met elkaar. Ze delen alleen de **Database Store** (`/internal/store`).

-----

## 🚀 Getting Started

Volg deze stappen om de backend lokaal op te zetten en te draaien.

### 1\. Vereisten

  * [Go](https://go.dev/doc/install) (v1.21 of hoger)
  * [Docker Desktop](https://www.docker.com/products/docker-desktop/) (voor Postgres)

### 2\. Configuratie (.env)

Maak een bestand genaamd `.env` in de `Backend` map door `.env.example` te kopiëren en de waarden aan te passen:

```bash
cp .env.example .env
```

**Pas de waarden aan** in `.env` (vooral de Google credentials en het wachtwoord).

```.env
#---------------------------------------------------
# 1. APPLICATIE CONFIGURATIE
#---------------------------------------------------
APP_ENV=development
API_PORT=8080

#---------------------------------------------------
# 2. DATABASE (POSTGRES)
#---------------------------------------------------
# Let op: we gebruiken poort 5433 om conflicten te vermijden
POSTGRES_USER=postgres
POSTGRES_PASSWORD=Bootje12
POSTGRES_DB=agenda_automator
POSTGRES_HOST=localhost
POSTGRES_PORT=5433

# De volledige URL voor de Go-applicatie
DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

#---------------------------------------------------
# 3. BEVEILIGING & ENCRYPTIE
#---------------------------------------------------
# Moet exact 32 karakters lang zijn (AES-256)
ENCRYPTION_KEY="IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea"

#---------------------------------------------------
# 5. OAUTH CLIENTS (Google)
#---------------------------------------------------
# Verkregen uit de Google Cloud Console
# De backend handelt de volledige OAuth flow af

# Je frontend (Next.js) draait waarschijnlijk op poort 3000
CLIENT_BASE_URL="http://localhost:3000"

# De backend callback URL (moet exact overeenkomen met Google Console)
OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/google/callback"

GOOGLE_OAUTH_CLIENT_ID="JOUW-CLIENT-ID-HIER.apps.googleusercontent.com"
GOOGLE_OAUTH_CLIENT_SECRET="JOUW-CLIENT-SECRET-HIER"
```

### 3\. Start de Database

Draai de `docker-compose.yml` om de Postgres container te starten.

```bash
# Start de container in de achtergrond
docker compose up -d
```

### 4\. Voer Database Migraties uit

Nu de database draait, voer je het SQL-schema uit om de tabellen aan te maken.

**Belangrijk:** Zorg dat je het commando aanpast met de poort (`5433`) en het wachtwoord uit je `.env` bestand.

```bash
migrate -database "postgres://postgres:Bootje12@localhost:5433/agenda_automator?sslmode=disable" -path db/migrations up
```

### 5\. Installeer Go Dependencies

Download alle benodigde Go-packages.

```bash
go mod tidy
```

### 6\. Draai de Applicatie

Start de Go-backend.

```bash
go run cmd/server/main.go
```

Je zou nu de volgende output moeten zien, wat aangeeft dat *zowel* de API als de Worker draaien:

```
2025/11/11 18:00:00 Successfully connected to database.
2025/11/11 18:00:00 Starting worker...
2025/11/11 18:00:00 Application starting API server on port 8080...
2025/11/11 18:00:00 [Worker] Running work cycle...
2025/11/11 18:00:00 [Worker] Found 0 active accounts to check.
```

-----

## 📖 API Endpoints

De API is beschikbaar op `http://localhost:8080` en vereist JWT authenticatie voor de meeste endpoints. Zie [API Reference](docs/API_REFERENCE.md) voor volledige documentatie.

### Health Check

  * **Endpoint:** `GET /api/v1/health`
  * **Omschrijving:** Controleert of de API-server draait.
  * **Response (200 OK):**
    ```json
    {"status":"ok"}
    ```

### Authentication

  * **Endpoint:** `GET /api/v1/auth/google/login`
  * **Omschrijving:** Start de Google OAuth flow. Genereert CSRF token en redirect naar Google.
  * **Response:** Redirect (307) naar Google OAuth.

  * **Endpoint:** `GET /api/v1/auth/google/callback`
  * **Omschrijving:** OAuth callback. Wisselt code voor tokens, maakt gebruiker aan, genereert JWT.
  * **Response:** Redirect (303) naar frontend met JWT token.

### User Management

  * **Endpoint:** `GET /api/v1/me` (vereist JWT)
  * **Omschrijving:** Haalt huidige gebruiker op.
  * **Response (200 OK):** Gebruiker object

### Connected Accounts

  * **Endpoint:** `GET /api/v1/accounts` (vereist JWT)
  * **Omschrijving:** Haalt alle gekoppelde accounts van de gebruiker op.
  * **Response (200 OK):** Array van connected account objecten

  * **Endpoint:** `DELETE /api/v1/accounts/{accountId}` (vereist JWT)
  * **Omschrijving:** Verwijdert een gekoppeld account.
  * **Response (204 No Content):**

### Automation Rules

  * **Endpoint:** `POST /api/v1/accounts/{accountId}/rules` (vereist JWT)
  * **Omschrijving:** Creëert een nieuwe automatiseringsregel.
  * **Response (201 Created):** Rule object

  * **Endpoint:** `GET /api/v1/accounts/{accountId}/rules` (vereist JWT)
  * **Omschrijving:** Haalt alle regels voor een account op.
  * **Response (200 OK):** Array van rule objecten

### Calendar Operations

  * **Endpoint:** `GET /api/v1/accounts/{accountId}/calendar/events` (vereist JWT)
  * **Omschrijving:** Haalt calendar events op.
  * **Response (200 OK):** Array van Google Calendar events

  * **Endpoint:** `POST /api/v1/accounts/{accountId}/calendar/events` (vereist JWT)
  * **Omschrijving:** Creëert een nieuw calendar event.
  * **Response (201 Created):** Created event object

-----

## 📁 Projectstructuur

  * `/cmd/server/main.go`: Het startpunt van de applicatie. Initialiseert de DB, Store, Worker en API-server.
  * `/db/migrations`: Bevat de SQL-bestanden (`.up.sql`, `.down.sql`) voor het database-schema.
  * `/internal/api`: Bevat de Chi-router (`server.go`), de HTTP-handlers (`handlers.go`) en JSON-helpers (`json.go`).
  * `/internal/crypto`: Bevat de AES-GCM logica (`crypto.go`) voor het versleutelen en ontsleutelen van de OAuth-tokens.
  * `/internal/database`: Bevat de `ConnectDB` functie voor het opzetten van de `pgxpool` connectie.
  * `/internal/domain`: Bevat de Go `structs` (`models.go`) die de databasetabellen vertegenwoordigen (bijv. `User`, `ConnectedAccount`).
  * `/internal/store`: Bevat de "Repository Pattern" (`store.go`). Dit is de *enige* plek waar SQL-queries worden uitgevoerd.
  * `/internal/worker`: Bevat de logica voor de achtergrond-worker (`worker.go`), inclusief token-verversing en de Google Calendar API-aanroepen.
  * `/docker-compose.yml`: Definieert de `postgres` service voor de lokale ontwikkelomgeving.
  * `/docs`: (Leeg) Gereserveerd voor meer diepgaande documentatie, zoals architectuurdiagrammen.
