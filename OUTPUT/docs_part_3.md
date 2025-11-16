# Documentation Part 3

Generated at: 2025-11-15T20:45:21+01:00

## internal\worker\worker.go

```go
package worker

import (
	// "agenda-automator-api/internal/crypto" // <-- VERWIJDERD
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// VERWIJDERD:
// var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

// Worker is de struct voor onze achtergrond-processor
type Worker struct {
	store store.Storer
	// googleOAuthConfig *oauth2.Config // <-- VERWIJDERD (zit nu in store)
}

// NewWorker (AANGEPAST)
func NewWorker(s store.Storer) (*Worker, error) {
	return &Worker{
		store: s,
	}, nil
}

// Start lanceert de worker in een aparte goroutine
func (w *Worker) Start() {
	log.Println("Starting worker...")

	go w.run()
}

// run is de hoofdloop die periodiek de accounts controleert (real-time monitoring)
func (w *Worker) run() {
	// Verhoogd interval om API-limieten te respecteren
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// Draai één keer direct bij het opstarten
	w.doWork()

	for {
		<-ticker.C
		w.doWork()
	}
}

// doWork is de daadwerkelijke werklading
func (w *Worker) doWork() {
	log.Println("[Worker] Running work cycle...")

	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second) // Iets korter dan ticker
	defer cancel()

	err := w.checkAccounts(ctx)
	if err != nil {
		log.Printf("[Worker] ERROR checking accounts: %v", err)
	}
}

// checkAccounts haalt alle accounts op, beheert tokens, en start de verwerking (parallel)
func (w *Worker) checkAccounts(ctx context.Context) error {
	accounts, err := w.store.GetActiveAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not get active accounts: %w", err)
	}

	log.Printf("[Worker] Found %d active accounts to check.", len(accounts))

	var wg sync.WaitGroup
	for _, acc := range accounts {
		wg.Add(1)
		go func(acc domain.ConnectedAccount) {
			defer wg.Done()
			w.processAccount(ctx, acc)
		}(acc)
	}
	wg.Wait()

	return nil
}

// processAccount (ZWAAR VEREENVOUDIGD)
func (w *Worker) processAccount(ctx context.Context, acc domain.ConnectedAccount) {
	// 1. Haal een gegarandeerd geldig token op.
	// De store regelt de decryptie, check, refresh, en update.
	token, err := w.store.GetValidTokenForAccount(ctx, acc.ID)
	if err != nil {
		// De store heeft de 'revoked' status al ingesteld,
		// we hoeven hier alleen nog maar te loggen en stoppen.
		if errors.Is(err, store.ErrTokenRevoked) {
			log.Printf("[Worker] Account %s is revoked. Stopping processing.", acc.ID)
		} else {
			log.Printf("[Worker] ERROR: Kon geen geldig token krijgen voor account %s: %v", acc.ID, err)
		}
		return // Stop verwerking voor dit account
	}

	// 2. Process calendar
	log.Printf("[Worker] Token for account %s is valid. Processing calendar...", acc.ID)
	if err := w.processCalendarEvents(ctx, acc, token); err != nil {
		log.Printf("[Worker] ERROR processing calendar for account %s: %v", acc.ID, err)
	}

	// 3. Update last_checked
	if err := w.store.UpdateAccountLastChecked(ctx, acc.ID); err != nil {
		log.Printf("[Worker] ERROR updating last_checked for account %s: %v", acc.ID, err)
	}
}

// VERWIJDERD: getTokenForAccount
// VERWIJDERD: refreshAccountToken

// eventExists (Bestaande code)
func eventExists(srv *calendar.Service, start, end time.Time, title string) bool {
	// Zoek in een iets ruimer venster om afrondingsfouten te vangen
	timeMin := start.Add(-1 * time.Minute).Format(time.RFC3339)
	timeMax := end.Add(1 * time.Minute).Format(time.RFC3339)

	events, err := srv.Events.List("primary").
		TimeMin(timeMin).
		TimeMax(timeMax).
		SingleEvents(true).
		Do()

	if err != nil {
		log.Printf("[Worker] ERROR checking for existing event: %v", err)
		// Veilige aanname: ga ervan uit dat het niet bestaat
		return false
	}

	for _, item := range events.Items {
		// Controleer op exacte titel-match
		if item.Summary == title {
			return true
		}
	}

	return false
}

// processCalendarEvents (Bestaande code)
func (w *Worker) processCalendarEvents(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {

	// De OAuth client is nu niet meer nodig in de worker,
	// maar wel om de calendar service te maken.
	// We halen hem op uit de store config.
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// Ongelimiteerd: Haal alle events op
	tMin := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	tMax := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

	log.Printf("[Worker] Fetching all calendar events for %s (unlimited)", acc.Email)

	events, err := srv.Events.List("primary").
		TimeMin(tMin).
		TimeMax(tMax).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(2500).
		Do()
	if err != nil {
		return fmt.Errorf("could not fetch calendar events: %w", err)
	}

	rules, err := w.store.GetRulesForAccount(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("could not fetch automation rules: %w", err)
	}

	if len(events.Items) == 0 || len(rules) == 0 {
		log.Printf("[Worker] No upcoming events or no rules found for %s. Skipping.", acc.Email)
		return nil
	}

	log.Printf("[Worker] Checking %d events against %d rules for %s...", len(events.Items), len(rules), acc.Email)

	for _, event := range events.Items {

		// Voorkom dat we reageren op onze eigen aangemaakte events
		if strings.HasPrefix(event.Description, "Automatische reminder voor:") {
			continue
		}

		// (Aangepast: filter op actieve regels gebeurt nu in de DB query)
		for _, rule := range rules {
			// (Nieuwe check: de query haalt nu *alle* regels op, we moeten inactieve skippen)
			if !rule.IsActive {
				continue
			}

			var trigger domain.TriggerConditions
			if err := json.Unmarshal(rule.TriggerConditions, &trigger); err != nil {
				log.Printf("[Worker] ERROR unmarshaling trigger for rule %s: %v", rule.ID, err)
				continue
			}

			// --- 1. CHECK TRIGGERS ---
			summaryMatch := false
			if trigger.SummaryEquals != "" && event.Summary == trigger.SummaryEquals {
				summaryMatch = true
			}
			if !summaryMatch && len(trigger.SummaryContains) > 0 {
				for _, contain := range trigger.SummaryContains {
					if strings.Contains(event.Summary, contain) {
						summaryMatch = true
						break
					}
				}
			}
			if !summaryMatch {
				continue
			}

			locationMatch := false
			if len(trigger.LocationContains) == 0 {
				locationMatch = true
			} else {
				eventLocationLower := strings.ToLower(event.Location)
				for _, loc := range trigger.LocationContains {
					if strings.Contains(eventLocationLower, strings.ToLower(loc)) {
						locationMatch = true
						break
					}
				}
			}
			if !locationMatch {
				continue
			}

			// --- 1.5. CHECK LOGS (OPTIMALISATIE 1) ---
			hasLogged, err := w.store.HasLogForTrigger(ctx, rule.ID, event.Id)
			if err != nil {
				log.Printf("[Worker] ERROR checking logs for event %s / rule %s: %v", event.Id, rule.ID, err)
			}
			if hasLogged {
				continue
			}
			// --- EINDE OPTIMALISATIE 1.5 ---

			// --- 2. VOER ACTIE UIT ---
			log.Printf("[Worker] MATCH: Event '%s' (ID: %s) matches rule '%s'.", event.Summary, event.Id, rule.Name)

			var action domain.ActionParams
			if err := json.Unmarshal(rule.ActionParams, &action); err != nil {
				log.Printf("[Worker] ERROR unmarshaling action for rule %s: %v", rule.ID, err)
				continue
			}

			if action.NewEventTitle == "" {
				log.Printf("[Worker] ERROR: Rule %s heeft geen 'new_event_title'.", rule.ID)
				continue
			}

			startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
			if err != nil {
				log.Printf("[Worker] ERROR parsing start time: %v", err)
				continue
			}

			offset := action.OffsetMinutes
			if offset == 0 {
				offset = -60
			}
			reminderTime := startTime.Add(time.Duration(offset) * time.Minute)

			durMin := action.DurationMin
			if durMin == 0 {
				durMin = 5
			}
			endTime := reminderTime.Add(time.Duration(durMin) * time.Minute)

			title := action.NewEventTitle

			// --- 3. CONTROLEER OP DUPLICATEN (SECUNDAIRE CHECK) ---
			if eventExists(srv, reminderTime, endTime, title) {
				log.Printf("[Worker] SKIP: Reminder event '%s' at %s already exists.", title, reminderTime)

				// --- 3.1. LOG DIT VOOR DE VOLGENDE KEER (OPTIMALISATIE 1) ---
				triggerDetailsJSON, _ := json.Marshal(domain.TriggerLogDetails{
					GoogleEventID:  event.Id,
					TriggerSummary: event.Summary,
					TriggerTime:    startTime,
				})
				actionDetailsJSON, _ := json.Marshal(domain.ActionLogDetails{
					CreatedEventID:      "unknown-pre-existing",
					CreatedEventSummary: title,
					ReminderTime:        reminderTime,
				})
				logParams := store.CreateLogParams{
					ConnectedAccountID: acc.ID,
					RuleID:             rule.ID,
					Status:             domain.LogSkipped,
					TriggerDetails:     triggerDetailsJSON,
					ActionDetails:      actionDetailsJSON,
				}
				if err := w.store.CreateAutomationLog(ctx, logParams); err != nil {
					log.Printf("[Worker] ERROR saving skip log for rule %s: %v", rule.ID, err)
				}
				continue
			}

			// --- 4. MAAK EVENT AAN ---
			newEvent := &calendar.Event{
				Summary: title,
				Start: &calendar.EventDateTime{
					DateTime: reminderTime.Format(time.RFC3339),
					TimeZone: event.Start.TimeZone,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime.Format(time.RFC3339),
					TimeZone: event.End.TimeZone,
				},
				Description: fmt.Sprintf("Automatische reminder voor: %s\nGemaakt door regel: %s", event.Summary, rule.Name),
			}

			createdEvent, err := srv.Events.Insert("primary", newEvent).Do()
			if err != nil {
				log.Printf("[Worker] ERROR creating reminder event: %v", err)

				// --- 4.1 LOG FAILURE (OPTIMALISATIE 1) ---
				triggerDetailsJSON, _ := json.Marshal(domain.TriggerLogDetails{
					GoogleEventID:  event.Id,
					TriggerSummary: event.Summary,
					TriggerTime:    startTime,
				})
				logParams := store.CreateLogParams{
					ConnectedAccountID: acc.ID,
					RuleID:             rule.ID,
					Status:             domain.LogFailure,
					TriggerDetails:     triggerDetailsJSON,
					ErrorMessage:       err.Error(),
				}
				if err := w.store.CreateAutomationLog(ctx, logParams); err != nil {
					log.Printf("[Worker] ERROR saving failure log for rule %s: %v", rule.ID, err)
				}
				continue
			}

			// --- 5. LOG SUCCESS (OPTIMALISATIE 1) ---
			triggerDetailsJSON, _ := json.Marshal(domain.TriggerLogDetails{
				GoogleEventID:  event.Id,
				TriggerSummary: event.Summary,
				TriggerTime:    startTime,
			})
			actionDetailsJSON, _ := json.Marshal(domain.ActionLogDetails{
				CreatedEventID:      createdEvent.Id,
				CreatedEventSummary: createdEvent.Summary,
				ReminderTime:        reminderTime,
			})

			logParams := store.CreateLogParams{
				ConnectedAccountID: acc.ID,
				RuleID:             rule.ID,
				Status:             domain.LogSuccess,
				TriggerDetails:     triggerDetailsJSON,
				ActionDetails:      actionDetailsJSON,
			}
			if err := w.store.CreateAutomationLog(ctx, logParams); err != nil {
				log.Printf("[Worker] ERROR saving success log for rule %s: %v", rule.ID, err)
			}

			log.Printf("[Worker] SUCCESS: Created reminder '%s' (ID: %s) for event '%s' (ID: %s)", createdEvent.Summary, createdEvent.Id, event.Summary, event.Id)
		}
	}

	return nil
}

```

## README.md

```markdown
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

```

## go.sum

```text
cloud.google.com/go/auth v0.17.0 h1:74yCm7hCj2rUyyAocqnFzsAYXgJhrG26XCFimrc/Kz4=
cloud.google.com/go/auth v0.17.0/go.mod h1:6wv/t5/6rOPAX4fJiRjKkJCvswLwdet7G8+UGXt7nCQ=
cloud.google.com/go/auth/oauth2adapt v0.2.8 h1:keo8NaayQZ6wimpNSmW5OPc283g65QNIiLpZnkHRbnc=
cloud.google.com/go/auth/oauth2adapt v0.2.8/go.mod h1:XQ9y31RkqZCcwJWNSx2Xvric3RrU88hAYYbjDWYDL+c=
cloud.google.com/go/compute/metadata v0.9.0 h1:pDUj4QMoPejqq20dK0Pg2N4yG9zIkYGdBtwLoEkH9Zs=
cloud.google.com/go/compute/metadata v0.9.0/go.mod h1:E0bWwX5wTnLPedCKqk3pJmVgCBSM6qQI1yTBdEb3C10=
github.com/davecgh/go-spew v1.1.0/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/felixge/httpsnoop v1.0.4 h1:NFTV2Zj1bL4mc9sqWACXbQFVBBg2W3GPvqp8/ESS2Wg=
github.com/felixge/httpsnoop v1.0.4/go.mod h1:m8KPJKqk1gH5J9DgRY2ASl2lWCfGKXixSwevea8zH2U=
github.com/go-chi/chi/v5 v5.2.3 h1:WQIt9uxdsAbgIYgid+BpYc+liqQZGMHRaUwp0JUcvdE=
github.com/go-chi/chi/v5 v5.2.3/go.mod h1:L2yAIGWB3H+phAw1NxKwWM+7eUH/lU8pOMm5hHcoops=
github.com/go-chi/cors v1.2.2 h1:Jmey33TE+b+rB7fT8MUy1u0I4L+NARQlK6LhzKPSyQE=
github.com/go-chi/cors v1.2.2/go.mod h1:sSbTewc+6wYHBBCW7ytsFSn836hqM7JxpglAy2Vzc58=
github.com/go-logr/logr v1.2.2/go.mod h1:jdQByPbusPIv2/zmleS9BjJVeZ6kBagPoEUsqbVz/1A=
github.com/go-logr/logr v1.4.3 h1:CjnDlHq8ikf6E492q6eKboGOC0T8CDaOvkHCIg8idEI=
github.com/go-logr/logr v1.4.3/go.mod h1:9T104GzyrTigFIr8wt5mBrctHMim0Nb2HLGrmQ40KvY=
github.com/go-logr/stdr v1.2.2 h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=
github.com/go-logr/stdr v1.2.2/go.mod h1:mMo/vtBO5dYbehREoey6XUKy/eSumjCCveDpRre4VKE=
github.com/golang-jwt/jwt/v5 v5.3.0 h1:pv4AsKCKKZuqlgs5sUmn4x8UlGa0kEVt/puTpKx9vvo=
github.com/golang-jwt/jwt/v5 v5.3.0/go.mod h1:fxCRLWMO43lRc8nhHWY6LGqRcf+1gQWArsqaEUEa5bE=
github.com/golang/protobuf v1.5.4 h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=
github.com/golang/protobuf v1.5.4/go.mod h1:lnTiLA8Wa4RWRcIUkrtSVa5nRhsEGBg48fD6rSs7xps=
github.com/google/go-cmp v0.7.0 h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=
github.com/google/go-cmp v0.7.0/go.mod h1:pXiqmnSA92OHEEa9HXL2W4E7lf9JzCmGVUdgjX3N/iU=
github.com/google/s2a-go v0.1.9 h1:LGD7gtMgezd8a/Xak7mEWL0PjoTQFvpRudN895yqKW0=
github.com/google/s2a-go v0.1.9/go.mod h1:YA0Ei2ZQL3acow2O62kdp9UlnvMmU7kA6Eutn0dXayM=
github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=
github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/googleapis/enterprise-certificate-proxy v0.3.7 h1:zrn2Ee/nWmHulBx5sAVrGgAa0f2/R35S4DJwfFaUPFQ=
github.com/googleapis/enterprise-certificate-proxy v0.3.7/go.mod h1:MkHOF77EYAE7qfSuSS9PU6g4Nt4e11cnsDUowfwewLA=
github.com/googleapis/gax-go/v2 v2.15.0 h1:SyjDc1mGgZU5LncH8gimWo9lW1DtIfPibOG81vgd/bo=
github.com/googleapis/gax-go/v2 v2.15.0/go.mod h1:zVVkkxAQHa1RQpg9z2AUCMnKhi0Qld9rcmyfL1OZhoc=
github.com/jackc/pgpassfile v1.0.0 h1:/6Hmqy13Ss2zCq62VdNG8tM1wchn8zjSGOBJ6icpsIM=
github.com/jackc/pgpassfile v1.0.0/go.mod h1:CEx0iS5ambNFdcRtxPj5JhEz+xB6uRky5eyVu/W2HEg=
github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 h1:iCEnooe7UlwOQYpKFhBabPMi4aNAfoODPEFNiAnClxo=
github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761/go.mod h1:5TJZWKEWniPve33vlWYSoGYefn3gLQRzjfDlhSJ9ZKM=
github.com/jackc/pgx/v5 v5.7.6 h1:rWQc5FwZSPX58r1OQmkuaNicxdmExaEz5A2DO2hUuTk=
github.com/jackc/pgx/v5 v5.7.6/go.mod h1:aruU7o91Tc2q2cFp5h4uP3f6ztExVpyVv88Xl/8Vl8M=
github.com/jackc/puddle/v2 v2.2.2 h1:PR8nw+E/1w0GLuRFSmiioY6UooMp6KJv0/61nB7icHo=
github.com/jackc/puddle/v2 v2.2.2/go.mod h1:vriiEXHvEE654aYKXXjOvZM39qJ0q+azkZFrfEOc3H4=
github.com/joho/godotenv v1.5.1 h1:7eLL/+HRGLY0ldzfGMeQkb7vMd0as4CfYvUVzLqw0N0=
github.com/joho/godotenv v1.5.1/go.mod h1:f4LDr5Voq0i2e/R5DDNOoa2zzDfwtkZa6DnEwAbqwq4=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
github.com/stretchr/objx v0.1.0/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
github.com/stretchr/testify v1.3.0/go.mod h1:M5WIy9Dh21IEIfnGCwXGc5bZfKNJtfHm1UVUgZn+9EI=
github.com/stretchr/testify v1.7.0/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.10.0 h1:Xv5erBjTwe/5IxqUQTdXv5kgmIvbHo3QQyRwhJsOfJA=
github.com/stretchr/testify v1.10.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
go.opentelemetry.io/auto/sdk v1.1.0 h1:cH53jehLUN6UFLY71z+NDOiNJqDdPRaXzTel0sJySYA=
go.opentelemetry.io/auto/sdk v1.1.0/go.mod h1:3wSPjt5PWp2RhlCcmmOial7AvC4DQqZb7a7wCow3W8A=
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 h1:F7Jx+6hwnZ41NSFTO5q4LYDtJRXBf2PD0rNBkeB/lus=
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0/go.mod h1:UHB22Z8QsdRDrnAtX4PntOl36ajSxcdUMt1sF7Y6E7Q=
go.opentelemetry.io/otel v1.37.0 h1:9zhNfelUvx0KBfu/gb+ZgeAfAgtWrfHJZcAqFC228wQ=
go.opentelemetry.io/otel v1.37.0/go.mod h1:ehE/umFRLnuLa/vSccNq9oS1ErUlkkK71gMcN34UG8I=
go.opentelemetry.io/otel/metric v1.37.0 h1:mvwbQS5m0tbmqML4NqK+e3aDiO02vsf/WgbsdpcPoZE=
go.opentelemetry.io/otel/metric v1.37.0/go.mod h1:04wGrZurHYKOc+RKeye86GwKiTb9FKm1WHtO+4EVr2E=
go.opentelemetry.io/otel/sdk v1.37.0 h1:ItB0QUqnjesGRvNcmAcU0LyvkVyGJ2xftD29bWdDvKI=
go.opentelemetry.io/otel/sdk v1.37.0/go.mod h1:VredYzxUvuo2q3WRcDnKDjbdvmO0sCzOvVAiY+yUkAg=
go.opentelemetry.io/otel/sdk/metric v1.37.0 h1:90lI228XrB9jCMuSdA0673aubgRobVZFhbjxHHspCPc=
go.opentelemetry.io/otel/sdk/metric v1.37.0/go.mod h1:cNen4ZWfiD37l5NhS+Keb5RXVWZWpRE+9WyVCpbo5ps=
go.opentelemetry.io/otel/trace v1.37.0 h1:HLdcFNbRQBE2imdSEgm/kwqmQj1Or1l/7bW6mxVK7z4=
go.opentelemetry.io/otel/trace v1.37.0/go.mod h1:TlgrlQ+PtQO5XFerSPUYG0JSgGyryXewPGyayAWSBS0=
golang.org/x/crypto v0.43.0 h1:dduJYIi3A3KOfdGOHX8AVZ/jGiyPa3IbBozJ5kNuE04=
golang.org/x/crypto v0.43.0/go.mod h1:BFbav4mRNlXJL4wNeejLpWxB7wMbc79PdRGhWKncxR0=
golang.org/x/net v0.46.0 h1:giFlY12I07fugqwPuWJi68oOnpfqFnJIJzaIIm2JVV4=
golang.org/x/net v0.46.0/go.mod h1:Q9BGdFy1y4nkUwiLvT5qtyhAnEHgnQ/zd8PfU6nc210=
golang.org/x/oauth2 v0.33.0 h1:4Q+qn+E5z8gPRJfmRy7C2gGG3T4jIprK6aSYgTXGRpo=
golang.org/x/oauth2 v0.33.0/go.mod h1:lzm5WQJQwKZ3nwavOZ3IS5Aulzxi68dUSgRHujetwEA=
golang.org/x/sync v0.18.0 h1:kr88TuHDroi+UVf+0hZnirlk8o8T+4MrK6mr60WkH/I=
golang.org/x/sync v0.18.0/go.mod h1:9KTHXmSnoGruLpwFjVSX0lNNA75CykiMECbovNTZqGI=
golang.org/x/sys v0.37.0 h1:fdNQudmxPjkdUTPnLn5mdQv7Zwvbvpaxqs831goi9kQ=
golang.org/x/sys v0.37.0/go.mod h1:OgkHotnGiDImocRcuBABYBEXf8A9a87e/uXjp9XT3ks=
golang.org/x/text v0.30.0 h1:yznKA/E9zq54KzlzBEAWn1NXSQ8DIp/NYMy88xJjl4k=
golang.org/x/text v0.30.0/go.mod h1:yDdHFIX9t+tORqspjENWgzaCVXgk0yYnYuSZ8UzzBVM=
gonum.org/v1/gonum v0.16.0 h1:5+ul4Swaf3ESvrOnidPp4GZbzf0mxVQpDCYUQE7OJfk=
gonum.org/v1/gonum v0.16.0/go.mod h1:fef3am4MQ93R2HHpKnLk4/Tbh/s0+wqD5nfa6Pnwy4E=
google.golang.org/api v0.256.0 h1:u6Khm8+F9sxbCTYNoBHg6/Hwv0N/i+V94MvkOSor6oI=
google.golang.org/api v0.256.0/go.mod h1:KIgPhksXADEKJlnEoRa9qAII4rXcy40vfI8HRqcU964=
google.golang.org/genproto v0.0.0-20250603155806-513f23925822 h1:rHWScKit0gvAPuOnu87KpaYtjK5zBMLcULh7gxkCXu4=
google.golang.org/genproto v0.0.0-20250603155806-513f23925822/go.mod h1:HubltRL7rMh0LfnQPkMH4NPDFEWp0jw3vixw7jEM53s=
google.golang.org/genproto/googleapis/api v0.0.0-20250804133106-a7a43d27e69b h1:ULiyYQ0FdsJhwwZUwbaXpZF5yUE3h+RA+gxvBu37ucc=
google.golang.org/genproto/googleapis/api v0.0.0-20250804133106-a7a43d27e69b/go.mod h1:oDOGiMSXHL4sDTJvFvIB9nRQCGdLP1o/iVaqQK8zB+M=
google.golang.org/genproto/googleapis/rpc v0.0.0-20251103181224-f26f9409b101 h1:tRPGkdGHuewF4UisLzzHHr1spKw92qLM98nIzxbC0wY=
google.golang.org/genproto/googleapis/rpc v0.0.0-20251103181224-f26f9409b101/go.mod h1:7i2o+ce6H/6BluujYR+kqX3GKH+dChPTQU19wjRPiGk=
google.golang.org/grpc v1.76.0 h1:UnVkv1+uMLYXoIz6o7chp59WfQUYA2ex/BXQ9rHZu7A=
google.golang.org/grpc v1.76.0/go.mod h1:Ju12QI8M6iQJtbcsV+awF5a4hfJMLi4X0JLo94ULZ6c=
google.golang.org/protobuf v1.36.10 h1:AYd7cD/uASjIL6Q9LiTjz8JLcrh/88q5UObnmY3aOOE=
google.golang.org/protobuf v1.36.10/go.mod h1:HTf+CrKn2C3g5S8VImy6tdcUvCska2kB7j23XfzDpco=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=

```

## db\migrations\000001_initial_schema.up.sql

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Maak ENUM types alleen aan als ze nog niet bestaan
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'provider_type') THEN
        CREATE TYPE provider_type AS ENUM (
            'google',
            'microsoft'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_status') THEN
        CREATE TYPE account_status AS ENUM (
            'active',
            'revoked',
            'error',
            'paused'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'automation_log_status') THEN
        CREATE TYPE automation_log_status AS ENUM (
            'pending',
            'success',
            'failure',
            'skipped'
        );
    END IF;
END$$;

-- Maak tabellen alleen aan als ze nog niet bestaan
CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS connected_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider provider_type NOT NULL,
    email text NOT NULL,
    provider_user_id text NOT NULL,
    access_token bytea NOT NULL,
    refresh_token bytea,
    token_expiry timestamptz NOT NULL,
    scopes text[],
    status account_status NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    last_checked timestamptz,
    UNIQUE (user_id, provider, provider_user_id)
);
CREATE INDEX IF NOT EXISTS idx_connected_accounts_user_id ON connected_accounts(user_id);

CREATE TABLE IF NOT EXISTS automation_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    name text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    trigger_conditions jsonb NOT NULL,
    action_params jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_automation_rules_account_id ON automation_rules(connected_account_id) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS automation_logs (
    id bigserial PRIMARY KEY,
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    rule_id uuid REFERENCES automation_rules(id) ON DELETE SET NULL,
    timestamp timestamptz NOT NULL DEFAULT now(),
    status automation_log_status NOT NULL,
    trigger_details jsonb,
    action_details jsonb,
    error_message text
);
CREATE INDEX IF NOT EXISTS idx_automation_logs_account_id_timestamp ON automation_logs(connected_account_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_automation_logs_rule_id ON automation_logs(rule_id, timestamp DESC);
```

## internal\crypto\crypto.go

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// Encrypt versleutelt data met AES-GCM
func Encrypt(plaintext []byte) ([]byte, error) {
	key := []byte(os.Getenv("ENCRYPTION_KEY"))
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt ontcijfert data met AES-GCM
func Decrypt(ciphertext []byte) ([]byte, error) {
	key := []byte(os.Getenv("ENCRYPTION_KEY"))
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}

	return plaintext, nil
}

```

## internal\database\database.go

```go
package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectDB maakt en test een verbinding met de database.
func ConnectDB() (*pgxpool.Pool, error) {
	dbUrl := os.Getenv("DATABASE_URL")
	log.Println("Connecting to DB URL:", dbUrl)
	if dbUrl == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	log.Println("Successfully connected to database.")
	return pool, nil
}

```

## Dockerfile

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
```

