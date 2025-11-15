Kijk of alles klopt het moet 100% kloppen:



C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\cmd\server

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\cmd\server\main.go

package main



import (

    "context"

    // _ "embed" // <-- VERWIJDERD

    "log"

    "net/http"

    "os"

    "strings"



    // Interne packages

    "agenda-automator-api/internal/api"

    "agenda-automator-api/internal/database"

    "agenda-automator-api/internal/store"

    "agenda-automator-api/internal/worker"



    "agenda-automator-api/db/migrations" // <-- TOEGEVOEGD



    // Externe packages

    "github.com/joho/godotenv"

    "golang.org/x/oauth2"

    "golang.org/x/oauth2/google"

)



// VERWIJDERD:

// //go:embed"../db/migrations/000001_initial_schema.up.sql"

// var migrationFile string



func main() {

    // 1. Laad configuratie (.env)

    _ = godotenv.Load()



    // 2. Maak verbinding met de Database

    pool, err := database.ConnectDB()

    if err != nil {

        log.Fatalf("Could not connect to the database: %v", err)

    }

    defer pool.Close()



    // -----------------------------------------------------

    // AANGEPAST: Stap 2.5 - Voer migraties uit

    run := os.Getenv("RUN_MIGRATIONS")

    if strings.ToLower(run) == "true" {

        log.Println("Running database migrations...")

        // Gebruik de variabele uit de 'migrations' package:

        if _, err := pool.Exec(context.Background(), migrations.InitialSchemaUp); err != nil {

            log.Fatalf("Database migrations failed: %v", err)

        }

        log.Println("Database migrations applied successfully.")

    } else {

        log.Println("Skipping migrations (RUN_MIGRATIONS is not 'true')")

    }

    // -----------------------------------------------------



    // 3. Initialiseer de Gedeelde OAuth2 Config

    clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")

    clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")

    redirectURL := os.Getenv("OAUTH_REDIRECT_URL") // e.g. http://localhost:8080/api/v1/auth/google/callback



    if clientID == "" || clientSecret == "" || redirectURL == "" {

        log.Fatalf("GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET, of OAUTH_REDIRECT_URL is niet ingesteld in .env")

    }



    googleOAuthConfig := &oauth2.Config{

        ClientID:     clientID,

        ClientSecret: clientSecret,

        RedirectURL:  redirectURL,

        Endpoint:     google.Endpoint,

        Scopes: []string{

            "https://www.googleapis.com/auth/calendar.events",

            "https://www.googleapis.com/auth/userinfo.email",

            "https://www.googleapis.com/auth/userinfo.profile",

        },

    }



    // 4. Initialiseer de 'Store' Laag (AANGEPAST)

    // De store heeft nu de oauth config nodig om zelf tokens te verversen.

    dbStore := store.NewStore(pool, googleOAuthConfig)



    // 5. Initialiseer de Worker (geef de store mee)

    appWorker, err := worker.NewWorker(dbStore)

    if err != nil {

        log.Fatalf("Could not initialize worker: %v", err)

    }



    // 6. Start de Worker in de achtergrond

    appWorker.Start()



    // 7. Initialiseer de API Server (geef de store en config mee)

    apiServer := api.NewServer(dbStore, googleOAuthConfig)



    // 8. Start de HTTP Server (op de voorgrond)

    port := os.Getenv("API_PORT")

    if port == "" {

        port = "8080" // Default poort

    }



    log.Printf("Application starting API server on port %s...", port)



    if err := http.ListenAndServe(":"+port, apiServer.Router); err != nil {

        log.Fatalf("Could not start server: %v", err)

    }

}









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\db\migrations

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\db\migrations\000001_initial_schema.down.sql

-- Rollback initial schema migration



DROP INDEX IF EXISTS idx_automation_logs_rule_id;

DROP INDEX IF EXISTS idx_automation_logs_account_id_timestamp;

DROP TABLE IF EXISTS automation_logs;



DROP INDEX IF EXISTS idx_automation_rules_account_id;

DROP TABLE IF EXISTS automation_rules;



DROP INDEX IF EXISTS idx_connected_accounts_user_id;

DROP TABLE IF EXISTS connected_accounts;



DROP TABLE IF EXISTS users;



DROP TYPE IF EXISTS automation_log_status;

DROP TYPE IF EXISTS account_status;

DROP TYPE IF EXISTS provider_type;









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\db\migrations\000001_initial_schema.up.sql

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









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\db\migrations\embed.go

// db/migrations/embed.go



package migrations



import "embed"



//go:embed 000001_initial_schema.up.sql

var InitialSchemaUp string



//go:embed 000001_initial_schema.down.sql

var InitialSchemaDown string



// Optioneel: als je ALLE sql-bestanden als een bestandssysteem wilt:

//go:embed *.sql

var SQLFiles embed.FS











C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\docs

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\api

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\api\handlers.go
package api

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// --- AUTH HANDLERS (Bestaande code) ---

const oauthStateCookieName = "oauthstate"

// HELPER: Haalt de user ID op die door de middleware in de context is gezet
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(userContextKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("missing or invalid user ID in context")
	}
	return userID, nil
}

// generateJWT creëert een nieuw JWT token voor een gebruiker
func generateJWT(userID uuid.UUID) (string, error) {
	jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))
	if len(jwtKey) == 0 {
		return "", fmt.Errorf("JWT_SECRET_KEY is niet ingesteld")
	}

	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"iss":     "agenda-automator-api",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 dagen geldig
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", fmt.Errorf("kon token niet ondertekenen: %w", err)
	}

	return tokenString, nil
}

// handleGoogleLogin start de OAuth-flow
func (s *Server) handleGoogleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 32)
		rand.Read(b)
		state := base64.URLEncoding.EncodeToString(b)

		cookie := &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   60 * 10,
		}
		http.SetCookie(w, cookie)

		authURL := s.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// handleGoogleCallback is het endpoint dat Google aanroept na de login
func (s *Server) handleGoogleCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Geen state cookie")
			return
		}
		if r.URL.Query().Get("state") != stateCookie.Value {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige state token")
			return
		}

		code := r.URL.Query().Get("code")
		token, err := s.googleOAuthConfig.Exchange(ctx, code)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon code niet inwisselen: %s", err.Error()))
			return
		}
		if token.RefreshToken == "" {
			WriteJSONError(w, http.StatusBadRequest, "Geen refresh token ontvangen. Probeer opnieuw.")
			return
		}

		userInfo, err := getUserInfo(ctx, token)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruikersinfo niet ophalen: %s", err.Error()))
			return
		}

		user, err := s.store.CreateUser(ctx, userInfo.Email, userInfo.Name)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruiker niet aanmaken: %s", err.Error()))
			return
		}

		params := store.UpsertConnectedAccountParams{
			UserID:         user.ID,
			Provider:       domain.ProviderGoogle,
			Email:          userInfo.Email,
			ProviderUserID: userInfo.Id,
			AccessToken:    token.AccessToken,
			RefreshToken:   token.RefreshToken,
			TokenExpiry:    token.Expiry,
			Scopes:         s.googleOAuthConfig.Scopes,
		}

		account, err := s.store.UpsertConnectedAccount(ctx, params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon account niet koppelen: %s", err.Error()))
			return
		}

		log.Printf("Account %s gekoppeld voor user %s", account.ID, user.ID)

		jwtString, err := generateJWT(user.ID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon authenticatie-token niet genereren: %s", err.Error()))
			return
		}

		redirectURL := fmt.Sprintf("%s/dashboard?token=%s", os.Getenv("CLIENT_BASE_URL"), jwtString)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

// getUserInfo haalt profielinfo op met een geldig token
func getUserInfo(ctx context.Context, token *oauth2.Token) (*oauth2v2.Userinfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	oauth2Service, err := oauth2v2.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

// --- USER HANDLERS ---

// handleGetMe haalt de gegevens op van de ingelogde gebruiker.
func (s *Server) handleGetMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		user, err := s.store.GetUserByID(r.Context(), userID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon gebruiker niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, user)
	}
}

// --- ACCOUNT HANDLERS ---

// handleGetConnectedAccounts haalt alle gekoppelde accounts op voor de gebruiker.
func (s *Server) handleGetConnectedAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		accounts, err := s.store.GetAccountsForUser(r.Context(), userID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon accounts niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, accounts)
	}
}

// handleDeleteConnectedAccount verwijdert een gekoppeld account.
func (s *Server) handleDeleteConnectedAccount() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		err = s.store.DeleteConnectedAccount(r.Context(), accountID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon account niet verwijderen")
			return
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// --- RULE HANDLERS ---

// handleCreateRule creëert een nieuwe automation rule.
func (s *Server) handleCreateRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		var req domain.AutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		params := store.CreateAutomationRuleParams{
			ConnectedAccountID: accountID,
			Name:               req.Name,
			TriggerConditions:  req.TriggerConditions,
			ActionParams:       req.ActionParams,
		}

		rule, err := s.store.CreateAutomationRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet creëren")
			return
		}

		WriteJSON(w, http.StatusCreated, rule)
	}
}

// handleGetRules haalt alle rules op voor een account.
func (s *Server) handleGetRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		rules, err := s.store.GetRulesForAccount(r.Context(), accountID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rules niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, rules)
	}
}

// handleUpdateRule update een bestaande rule.
func (s *Server) handleUpdateRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		rule, err := s.store.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden")
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule")
			return
		}

		var req domain.AutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		params := store.UpdateRuleParams{
			RuleID:            ruleID,
			Name:              req.Name,
			TriggerConditions: req.TriggerConditions,
			ActionParams:      req.ActionParams,
		}

		updatedRule, err := s.store.UpdateRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet updaten")
			return
		}

		WriteJSON(w, http.StatusOK, updatedRule)
	}
}

// handleDeleteRule verwijdert een rule.
func (s *Server) handleDeleteRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		rule, err := s.store.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden")
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule")
			return
		}

		err = s.store.DeleteRule(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet verwijderen")
			return
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// handleToggleRule togglet de active status van een rule.
func (s *Server) handleToggleRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		rule, err := s.store.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden")
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule")
			return
		}

		updatedRule, err := s.store.ToggleRuleStatus(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule status niet togglen")
			return
		}

		WriteJSON(w, http.StatusOK, updatedRule)
	}
}

// --- LOG HANDLERS ---

// handleGetAutomationLogs haalt logs op voor een account.
func (s *Server) handleGetAutomationLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		limit := 50 // Default limit
		logs, err := s.store.GetLogsForAccount(r.Context(), accountID, limit)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon logs niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, logs)
	}
}

// --- NIEUWE HANDLERS VOOR EVENT CRUD ---

// handleCreateEvent creëert een nieuw event in Google Calendar
func (s *Server) handleCreateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = "primary"
		}

		var req calendar.Event
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		createdEvent, err := client.Events.Insert(calendarID, &req).Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon event niet creëren: %v", err))
			return
		}

		WriteJSON(w, http.StatusCreated, createdEvent)
	}
}

// handleUpdateEvent update een bestaand event
func (s *Server) handleUpdateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		eventID := chi.URLParam(r, "eventId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = "primary"
		}

		var req calendar.Event
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		updatedEvent, err := client.Events.Update(calendarID, eventID, &req).Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon event niet updaten: %v", err))
			return
		}

		WriteJSON(w, http.StatusOK, updatedEvent)
	}
}

// handleDeleteEvent verwijdert een event
func (s *Server) handleDeleteEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		eventID := chi.URLParam(r, "eventId")
		calendarID := r.URL.Query().Get("calendarId") // Optioneel param voor secundaire calendar
		if calendarID == "" {
			calendarID = "primary"
		}

		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		err = client.Events.Delete(calendarID, eventID).Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon event niet verwijderen: %v", err))
			return
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// --- BIJGEWERKTE HANDLER VOOR EVENT FETCH (MET MULTI-CALENDAR SUPPORT) ---

// handleGetCalendarEvents haalt events op (nu met optionele calendarId param)
func (s *Server) handleGetCalendarEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Nieuw: Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = "primary"
		}

		timeMin := time.Now()
		timeMax := timeMin.AddDate(0, 3, 0) // 3 maanden vooruit (bestaand)

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		events, err := client.Events.List(calendarID).
			TimeMin(timeMin.Format(time.RFC3339)).
			TimeMax(timeMax.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime").
			Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon events niet ophalen: %v", err))
			return
		}

		WriteJSON(w, http.StatusOK, events.Items)
	}
}

// --- NIEUWE HANDLER VOOR AGGREGATED EVENTS ---

// handleGetAggregatedEvents haalt events op van meerdere accounts/calendars
func (s *Server) handleGetAggregatedEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Parse request body: lijst van {accountId, calendarId} pairs
		type AggRequest struct {
			Accounts []struct {
				AccountID  string `json:"accountId"`
				CalendarID string `json:"calendarId"`
			} `json:"accounts"`
		}
		var req AggRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		timeMin := time.Now()
		timeMax := timeMin.AddDate(0, 3, 0)

		var allEvents []*calendar.Event
		ctx := r.Context()

		for _, acc := range req.Accounts {
			accountID, err := uuid.Parse(acc.AccountID)
			if err != nil {
				continue // Skip invalid
			}

			account, err := s.store.GetConnectedAccountByID(ctx, accountID)
			if err != nil || account.UserID != userID {
				continue // Skip not found or not owned
			}

			client, err := s.getCalendarClient(ctx, account)
			if err != nil {
				continue
			}

			calID := acc.CalendarID
			if calID == "" {
				calID = "primary"
			}

			events, err := client.Events.List(calID).
				TimeMin(timeMin.Format(time.RFC3339)).
				TimeMax(timeMax.Format(time.RFC3339)).
				SingleEvents(true).
				OrderBy("startTime").
				Do()
			if err != nil {
				continue
			}

			allEvents = append(allEvents, events.Items...)
		}

		WriteJSON(w, http.StatusOK, allEvents)
	}
}

// --- HELPER FUNCTIE (NIEUW) ---

// getCalendarClient initialiseert een Google Calendar client met token refresh
func (s *Server) getCalendarClient(ctx context.Context, account domain.ConnectedAccount) (*calendar.Service, error) {

	// 1. Gebruik de centrale store-logica.
	// Deze functie doet ALLES: decrypten, checken, refreshen, en re-encrypten.
	token, err := s.store.GetValidTokenForAccount(ctx, account.ID)
	if err != nil {
		// De store handelt ook 'revoked' tokens af, dus we hoeven alleen de fout terug te geven.
		return nil, fmt.Errorf("kon geen geldig token voor account ophalen: %w", err)
	}

	// 2. We hebben nu een gegarandeerd geldig token. Gebruik het.
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	return calendar.NewService(ctx, option.WithHTTPClient(client))
}







C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\api\json.go

package api



import (

    "encoding/json"

    "log"

    "net/http"

)



// WriteJSON schrijft een standaard JSON response

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {

    w.Header().Set("Content-Type", "application/json")

    w.WriteHeader(status)

    if err := json.NewEncoder(w).Encode(data); err != nil {

        log.Printf("could not write json response: %v", err)

    }

}



// WriteJSONError schrijft een standaard JSON error response

func WriteJSONError(w http.ResponseWriter, status int, message string) {

    WriteJSON(w, status, map[string]string{"error": message})

}









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\api\server.go

package api



import (

    "agenda-automator-api/internal/store"

    "context"

    "fmt"

    "net/http"

    "os"

    "strings"



    "github.com/go-chi/chi/v5"

    "github.com/go-chi/cors"

    "github.com/golang-jwt/jwt/v5"

    "github.com/google/uuid"

    "golang.org/x/oauth2"

)



type contextKey string



var userContextKey contextKey = "user_id"



type Server struct {

    Router            *chi.Mux

    store             store.Storer

    googleOAuthConfig *oauth2.Config

}



func NewServer(s store.Storer, oauthConfig *oauth2.Config) *Server {

    server := &Server{

        Router:            chi.NewRouter(),

        store:             s,

        googleOAuthConfig: oauthConfig,

    }



    server.setupMiddleware()

    server.setupRoutes()



    return server

}



func (s *Server) setupMiddleware() {

    allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")

    s.Router.Use(cors.Handler(cors.Options{

        AllowedOrigins:   allowedOrigins,

        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},

        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},

        ExposedHeaders:   []string{"Link"},

        AllowCredentials: true,

        MaxAge:           300,

    }))

}



func (s *Server) setupRoutes() {

    s.Router.Route("/api/v1", func(r chi.Router) {

        // Auth routes (bestaand)

        r.Get("/auth/google/login", s.handleGoogleLogin())

        r.Get("/auth/google/callback", s.handleGoogleCallback())



        // Protected routes

        r.Group(func(r chi.Router) {

            r.Use(s.authMiddleware)



            // User routes (bestaand)

            r.Get("/me", s.handleGetMe())

            r.Get("/users/me", s.handleGetMe())



            // Account routes (bestaand)

            r.Get("/accounts", s.handleGetConnectedAccounts())

            r.Delete("/accounts/{accountId}", s.handleDeleteConnectedAccount())



            // Rule routes (bestaand)

            r.Post("/accounts/{accountId}/rules", s.handleCreateRule())

            r.Get("/accounts/{accountId}/rules", s.handleGetRules())

            r.Put("/rules/{ruleId}", s.handleUpdateRule())

            r.Delete("/rules/{ruleId}", s.handleDeleteRule())

            r.Put("/rules/{ruleId}/toggle", s.handleToggleRule())



            // Log routes (bestaand)

            r.Get("/accounts/{accountId}/logs", s.handleGetAutomationLogs())



            // Calendar routes (bijgewerkt + nieuw)

            r.Get("/accounts/{accountId}/calendar/events", s.handleGetCalendarEvents()) // Bijgewerkt voor multi-calendar



            // NIEUW: CRUD voor events

            r.Post("/accounts/{accountId}/calendar/events", s.handleCreateEvent())

            r.Put("/accounts/{accountId}/calendar/events/{eventId}", s.handleUpdateEvent())

            r.Delete("/accounts/{accountId}/calendar/events/{eventId}", s.handleDeleteEvent())



            // NIEUW: Aggregated events

            r.Post("/calendar/aggregated-events", s.handleGetAggregatedEvents())

        })

    })

}



// authMiddleware valideert JWT en zet user ID in context

func (s *Server) authMiddleware(next http.Handler) http.Handler {

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        authHeader := r.Header.Get("Authorization")

        if authHeader == "" {

            WriteJSONError(w, http.StatusUnauthorized, "Geen authenticatie header")

            return

        }



        tokenString := strings.TrimPrefix(authHeader, "Bearer ")

        jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))



        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {

            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {

                return nil, fmt.Errorf("ongeldige signing method")

            }

            return jwtKey, nil

        })



        if err != nil || !token.Valid {

            WriteJSONError(w, http.StatusUnauthorized, "Ongeldige token")

            return

        }



        claims, ok := token.Claims.(jwt.MapClaims)

        if !ok {

            WriteJSONError(w, http.StatusUnauthorized, "Ongeldige claims")

            return

        }



        userIDStr, ok := claims["user_id"].(string)

        if !ok {

            WriteJSONError(w, http.StatusUnauthorized, "Geen user ID in token")

            return

        }



        userID, err := uuid.Parse(userIDStr)

        if err != nil {

            WriteJSONError(w, http.StatusUnauthorized, "Ongeldig user ID")

            return

        }



        ctx := context.WithValue(r.Context(), userContextKey, userID)

        next.ServeHTTP(w, r.WithContext(ctx))

    })

}









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\crypto

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\crypto\crypto.go

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









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\database

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\database\database.go

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









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\domain

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\domain\models.go

package domain



import (

    "encoding/json" // Zorg dat deze import er is

    "time"



    "github.com/google/uuid"

)



// --- ENUM Types ---

type ProviderType string



const (

    ProviderGoogle    ProviderType = "google"

    ProviderMicrosoft ProviderType = "microsoft"

)



type AccountStatus string



const (

    StatusActive  AccountStatus = "active"

    StatusRevoked AccountStatus = "revoked"

    StatusError   AccountStatus = "error"

    StatusPaused  AccountStatus = "paused"

)



type AutomationLogStatus string



const (

    LogPending AutomationLogStatus = "pending"

    LogSuccess AutomationLogStatus = "success"

    LogFailure AutomationLogStatus = "failure"

    LogSkipped AutomationLogStatus = "skipped"

)



// --- Tabel Structs (met JSON tags) ---



type User struct {

    ID        uuid.UUID `db:"id"        json:"id"`

    Email     string    `db:"email"     json:"email"`

    Name      *string   `db:"name"      json:"name,omitempty"` // AANGEPAST: van 'string' naar '*string'

    CreatedAt time.Time `db:"created_at" json:"created_at"`

    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

}



type ConnectedAccount struct {

    ID             uuid.UUID     `db:"id"                 json:"id"`

    UserID         uuid.UUID     `db:"user_id"            json:"user_id"`

    Provider       ProviderType  `db:"provider"           json:"provider"`

    Email          string        `db:"email"              json:"email"`

    ProviderUserID string        `db:"provider_user_id"   json:"provider_user_id"`

    AccessToken    []byte        `db:"access_token"       json:"-"`

    RefreshToken   []byte        `db:"refresh_token"      json:"-"`

    TokenExpiry    time.Time     `db:"token_expiry"       json:"token_expiry"`

    Scopes         []string      `db:"scopes"             json:"scopes"`

    Status         AccountStatus `db:"status"             json:"status"`

    CreatedAt      time.Time     `db:"created_at"         json:"created_at"`

    UpdatedAt      time.Time     `db:"updated_at"         json:"updated_at"`

    LastChecked    *time.Time    `db:"last_checked"       json:"last_checked"`

}



type AutomationRule struct {

    ID                 uuid.UUID       `db:"id"                     json:"id"`

    ConnectedAccountID uuid.UUID       `db:"connected_account_id"   json:"connected_account_id"`

    Name               string          `db:"name"                   json:"name"`

    IsActive           bool            `db:"is_active"              json:"is_active"`

    TriggerConditions  json.RawMessage `db:"trigger_conditions"     json:"trigger_conditions"`

    ActionParams       json.RawMessage `db:"action_params"          json:"action_params"`

    CreatedAt          time.Time       `db:"created_at"             json:"created_at"`

    UpdatedAt          time.Time       `db:"updated_at"             json:"updated_at"`

}



type AutomationLog struct {

    ID                 int64               `db:"id"                     json:"id"`

    ConnectedAccountID uuid.UUID           `db:"connected_account_id"   json:"connected_account_id"`

    RuleID             uuid.UUID           `db:"rule_id"                json:"rule_id"`

    Timestamp          time.Time           `db:"timestamp"              json:"timestamp"`

    Status             AutomationLogStatus `db:"status"                 json:"status"`

    TriggerDetails     json.RawMessage     `db:"trigger_details"        json:"trigger_details"`

    ActionDetails      json.RawMessage     `db:"action_details"         json:"action_details"`

    ErrorMessage       string              `db:"error_message"          json:"error_message"`

}



// ... rest van het bestand (TriggerConditions, ActionParams, etc.) ...

// (Deze hoeven niet aangepast te worden)

type TriggerConditions struct {

    SummaryEquals    string   `json:"summary_equals,omitempty"`

    SummaryContains  []string `json:"summary_contains,omitempty"`

    LocationContains []string `json:"location_contains,omitempty"`

}



type ActionParams struct {

    OffsetMinutes int    `json:"offset_minutes"`

    NewEventTitle string `json:"new_event_title"`

    DurationMin   int    `json:"duration_min"`

}



type TriggerLogDetails struct {

    GoogleEventID  string    `json:"google_event_id"`

    TriggerSummary string    `json:"trigger_summary"`

    TriggerTime    time.Time `json:"trigger_time"`

}



type ActionLogDetails struct {

    CreatedEventID      string    `json:"created_event_id"`

    CreatedEventSummary string    `json:"created_event_summary"`

    ReminderTime        time.Time `json:"reminder_time"`

}



type Event struct {

    ID          string    `json:"id"`

    Summary     string    `json:"summary"`

    Description string    `json:"description"`

    Start       time.Time `json:"start"`

    End         time.Time `json:"end"`

    CalendarId  string    `json:"calendarId"`

}









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\logger

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\logger\logs

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\store

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\store\store.go

package store



import (

    "agenda-automator-api/internal/crypto"

    "agenda-automator-api/internal/domain"

    "context"

    "encoding/json"

    "errors"

    "fmt"

    "log"

    "strings"

    "time"



    "github.com/google/uuid"

    "github.com/jackc/pgx/v5"

    "github.com/jackc/pgx/v5/pgxpool"

    "golang.org/x/oauth2"

)



// ErrTokenRevoked wordt gegooid als de gebruiker de toegang heeft ingetrokken.

var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")



// Storer is de interface voor al onze database-interacties.

type Storer interface {

    CreateUser(ctx context.Context, email, name string) (domain.User, error)

    GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error)

    DeleteUser(ctx context.Context, userID uuid.UUID) error



    UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error)

    GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error)

    GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error)

    GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error)

    UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error

    UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error

    UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error

    DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error

    VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error



    CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error)

    GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error)

    GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error)

    UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error)

    ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error)

    DeleteRule(ctx context.Context, ruleID uuid.UUID) error

    VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error



    CreateAutomationLog(ctx context.Context, arg CreateLogParams) error

    HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error)

    GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error)



    // Gecentraliseerde Token Logica

    GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error)



    // UpdateConnectedAccountToken update access/refresh token

    UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error

}



// DBStore implementeert de Storer interface.

type DBStore struct {

    pool              *pgxpool.Pool

    googleOAuthConfig *oauth2.Config // Nodig om tokens te verversen

}



// NewStore maakt een nieuwe DBStore

func NewStore(pool *pgxpool.Pool, oauthCfg *oauth2.Config) Storer {

    return &DBStore{

        pool:              pool,

        googleOAuthConfig: oauthCfg,

    }

}



// --- USER FUNCTIES ---



// CreateUser maakt een nieuwe gebruiker aan in de database

func (s *DBStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {

    query := `

    INSERT INTO users (email, name)

    VALUES ($1, $2)

    ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name

    RETURNING id, email, name, created_at, updated_at;

    `



    row := s.pool.QueryRow(ctx, query, email, name)



    var u domain.User

    err := row.Scan(

        &u.ID,

        &u.Email,

        &u.Name,

        &u.CreatedAt,

        &u.UpdatedAt,

    )



    if err != nil {

        return domain.User{}, fmt.Errorf("db scan error: %w", err)

    }



    return u, nil

}



// GetUserByID haalt een gebruiker op basis van ID.

func (s *DBStore) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {

    query := `SELECT id, email, name, created_at, updated_at FROM users WHERE id = $1`

    row := s.pool.QueryRow(ctx, query, userID)



    var u domain.User

    err := row.Scan(

        &u.ID,

        &u.Email,

        &u.Name,

        &u.CreatedAt,

        &u.UpdatedAt,

    )



    if err != nil {

        if errors.Is(err, pgx.ErrNoRows) {

            return domain.User{}, fmt.Errorf("user not found")

        }

        return domain.User{}, fmt.Errorf("db scan error: %w", err)

    }



    return u, nil

}



// DeleteUser verwijdert een gebruiker en al zijn data (via ON DELETE CASCADE).

func (s *DBStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {

    query := `DELETE FROM users WHERE id = $1`

    cmdTag, err := s.pool.Exec(ctx, query, userID)

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }

    if cmdTag.RowsAffected() == 0 {

        return fmt.Errorf("no user found with ID %s to delete", userID)

    }

    return nil

}



// --- ACCOUNT FUNCTIES ---



// UpsertConnectedAccountParams (aangepast van Create)

type UpsertConnectedAccountParams struct {

    UserID         uuid.UUID

    Provider       domain.ProviderType

    Email          string

    ProviderUserID string

    AccessToken    string

    RefreshToken   string

    TokenExpiry    time.Time

    Scopes         []string

}



// UpsertConnectedAccount versleutelt de tokens en slaat het account op (upsert)

func (s *DBStore) UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {



    encryptedAccessToken, err := crypto.Encrypt([]byte(arg.AccessToken))

    if err != nil {

        return domain.ConnectedAccount{}, fmt.Errorf("could not encrypt access token: %w", err)

    }



    var encryptedRefreshToken []byte

    if arg.RefreshToken != "" {

        encryptedRefreshToken, err = crypto.Encrypt([]byte(arg.RefreshToken))

        if err != nil {

            return domain.ConnectedAccount{}, fmt.Errorf("could not encrypt refresh token: %w", err)

        }

    }



    query := `

    INSERT INTO connected_accounts (

        user_id, provider, email, provider_user_id,

        access_token, refresh_token, token_expiry, scopes, status

    ) VALUES (

        $1, $2, $3, $4, $5, $6, $7, $8, 'active'

    )

    ON CONFLICT (user_id, provider, provider_user_id) 

    DO UPDATE SET 

        access_token = EXCLUDED.access_token, 

        refresh_token = EXCLUDED.refresh_token, 

        token_expiry = EXCLUDED.token_expiry, 

        scopes = EXCLUDED.scopes, 

        status = 'active', 

        updated_at = now()

    RETURNING id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status, created_at, updated_at, last_checked;

    `



    row := s.pool.QueryRow(ctx, query,

        arg.UserID,

        arg.Provider,

        arg.Email,

        arg.ProviderUserID,

        encryptedAccessToken,

        encryptedRefreshToken,

        arg.TokenExpiry,

        arg.Scopes,

    )



    var acc domain.ConnectedAccount

    err = row.Scan(

        &acc.ID,

        &acc.UserID,

        &acc.Provider,

        &acc.Email,

        &acc.ProviderUserID,

        &acc.AccessToken,

        &acc.RefreshToken,

        &acc.TokenExpiry,

        &acc.Scopes,

        &acc.Status,

        &acc.CreatedAt,

        &acc.UpdatedAt,

        &acc.LastChecked,

    )



    if err != nil {

        return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)

    }



    return acc, nil

}



// GetConnectedAccountByID ...

func (s *DBStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {

    query := `

        SELECT id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status, created_at, updated_at, last_checked

        FROM connected_accounts

        WHERE id = $1

    `



    row := s.pool.QueryRow(ctx, query, id)



    var acc domain.ConnectedAccount

    err := row.Scan(

        &acc.ID,

        &acc.UserID,

        &acc.Provider,

        &acc.Email,

        &acc.ProviderUserID,

        &acc.AccessToken,

        &acc.RefreshToken,

        &acc.TokenExpiry,

        &acc.Scopes,

        &acc.Status,

        &acc.CreatedAt,

        &acc.UpdatedAt,

        &acc.LastChecked,

    )



    if err != nil {

        if errors.Is(err, pgx.ErrNoRows) {

            return domain.ConnectedAccount{}, fmt.Errorf("account not found")

        }

        return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)

    }



    return acc, nil

}



// UpdateAccountTokensParams ...

type UpdateAccountTokensParams struct {

    AccountID       uuid.UUID

    NewAccessToken  string

    NewRefreshToken string

    NewTokenExpiry  time.Time

}



// UpdateConnectedAccountTokenParams ...

type UpdateConnectedAccountTokenParams struct {

    ID           uuid.UUID

    AccessToken  []byte

    RefreshToken []byte

    TokenExpiry  time.Time

}



// UpdateAccountTokens ...

func (s *DBStore) UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error {

    encryptedAccessToken, err := crypto.Encrypt([]byte(arg.NewAccessToken))

    if err != nil {

        return fmt.Errorf("could not encrypt new access token: %w", err)

    }



    var query string

    var args []interface{}



    if arg.NewRefreshToken != "" {

        encryptedRefreshToken, err := crypto.Encrypt([]byte(arg.NewRefreshToken))

        if err != nil {

            return fmt.Errorf("could not encrypt new refresh token: %w", err)

        }



        query = `

        UPDATE connected_accounts

        SET access_token = $1, refresh_token = $2, token_expiry = $3, updated_at = now()

        WHERE id = $4;

        `

        args = []interface{}{encryptedAccessToken, encryptedRefreshToken, arg.NewTokenExpiry, arg.AccountID}

    } else {

        query = `

        UPDATE connected_accounts

        SET access_token = $1, token_expiry = $2, updated_at = now()

        WHERE id = $3;

        `

        args = []interface{}{encryptedAccessToken, arg.NewTokenExpiry, arg.AccountID}

    }



    cmdTag, err := s.pool.Exec(ctx, query, args...)

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }



    if cmdTag.RowsAffected() == 0 {

        return fmt.Errorf("no account found with ID %s to update", arg.AccountID)

    }



    return nil

}



// UpdateAccountLastChecked

func (s *DBStore) UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error {

    query := `

    UPDATE connected_accounts

    SET last_checked = now(), updated_at = now()

    WHERE id = $1;

    `



    _, err := s.pool.Exec(ctx, query, id)

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }



    return nil

}



// GetActiveAccounts haalt alle accounts op die de worker moet controleren

func (s *DBStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {

    query := `

    SELECT id, user_id, provider, email, provider_user_id,

           access_token, refresh_token, token_expiry, scopes, status,

           created_at, updated_at, last_checked

    FROM connected_accounts

    WHERE status = 'active';

    `



    rows, err := s.pool.Query(ctx, query)

    if err != nil {

        return nil, fmt.Errorf("db query error: %w", err)

    }

    defer rows.Close()



    var accounts []domain.ConnectedAccount

    for rows.Next() {

        var acc domain.ConnectedAccount

        err := rows.Scan(

            &acc.ID,

            &acc.UserID,

            &acc.Provider,

            &acc.Email,

            &acc.ProviderUserID,

            &acc.AccessToken,

            &acc.RefreshToken,

            &acc.TokenExpiry,

            &acc.Scopes,

            &acc.Status,

            &acc.CreatedAt,

            &acc.UpdatedAt,

            &acc.LastChecked,

        )

        if err != nil {

            return nil, fmt.Errorf("db row scan error: %w", err)

        }

        accounts = append(accounts, acc)

    }



    if err := rows.Err(); err != nil {

        return nil, fmt.Errorf("db rows error: %w", err)

    }



    return accounts, nil

}



// GetAccountsForUser haalt alle accounts op die eigendom zijn van een specifieke gebruiker

func (s *DBStore) GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) {

    query := `

    SELECT id, user_id, provider, email, provider_user_id,

           access_token, refresh_token, token_expiry, scopes, status,

           created_at, updated_at, last_checked

    FROM connected_accounts

    WHERE user_id = $1

    ORDER BY created_at DESC;

    `



    rows, err := s.pool.Query(ctx, query, userID)

    if err != nil {

        return nil, fmt.Errorf("db query error: %w", err)

    }

    defer rows.Close()



    var accounts []domain.ConnectedAccount

    for rows.Next() {

        var acc domain.ConnectedAccount

        err := rows.Scan(

            &acc.ID,

            &acc.UserID,

            &acc.Provider,

            &acc.Email,

            &acc.ProviderUserID,

            &acc.AccessToken,

            &acc.RefreshToken,

            &acc.TokenExpiry,

            &acc.Scopes,

            &acc.Status,

            &acc.CreatedAt,

            &acc.UpdatedAt,

            &acc.LastChecked,

        )

        if err != nil {

            return nil, fmt.Errorf("db row scan error: %w", err)

        }

        accounts = append(accounts, acc)

    }



    if err := rows.Err(); err != nil {

        return nil, fmt.Errorf("db rows error: %w", err)

    }



    return accounts, nil

}



// VerifyAccountOwnership controleert of een gebruiker eigenaar is van een account

func (s *DBStore) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error {

    query := `

    SELECT 1 

    FROM connected_accounts

    WHERE id = $1 AND user_id = $2

    LIMIT 1;

    `

    var exists int

    err := s.pool.QueryRow(ctx, query, accountID, userID).Scan(&exists)



    if err != nil {

        if errors.Is(err, pgx.ErrNoRows) {

            return fmt.Errorf("forbidden: account not found or does not belong to user")

        }

        return fmt.Errorf("db query error: %w", err)

    }



    return nil

}



// DeleteConnectedAccount verwijdert een specifiek account en diens data.

func (s *DBStore) DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error {

    query := `DELETE FROM connected_accounts WHERE id = $1`

    cmdTag, err := s.pool.Exec(ctx, query, accountID)

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }

    if cmdTag.RowsAffected() == 0 {

        return fmt.Errorf("no account found with ID %s to delete", accountID)

    }

    return nil

}



// --- RULE FUNCTIES ---



// CreateAutomationRuleParams

type CreateAutomationRuleParams struct {

    ConnectedAccountID uuid.UUID

    Name               string

    TriggerConditions  json.RawMessage // []byte

    ActionParams       json.RawMessage // []byte

}



// CreateAutomationRule

func (s *DBStore) CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error) {

    query := `

    INSERT INTO automation_rules (

        connected_account_id, name, trigger_conditions, action_params

    ) VALUES (

        $1, $2, $3, $4

    )

    RETURNING id, connected_account_id, name, is_active, trigger_conditions, action_params, created_at, updated_at;

    `



    row := s.pool.QueryRow(ctx, query,

        arg.ConnectedAccountID,

        arg.Name,

        arg.TriggerConditions,

        arg.ActionParams,

    )



    var rule domain.AutomationRule

    err := row.Scan(

        &rule.ID,

        &rule.ConnectedAccountID,

        &rule.Name,

        &rule.IsActive,

        &rule.TriggerConditions,

        &rule.ActionParams,

        &rule.CreatedAt,

        &rule.UpdatedAt,

    )



    if err != nil {

        return domain.AutomationRule{}, fmt.Errorf("db scan error: %w", err)

    }



    return rule, nil

}



// GetRuleByID ...

func (s *DBStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {

    query := `

        SELECT id, connected_account_id, name, is_active,

               trigger_conditions, action_params, created_at, updated_at

        FROM automation_rules

        WHERE id = $1

        `

    row := s.pool.QueryRow(ctx, query, ruleID)



    var rule domain.AutomationRule

    err := row.Scan(

        &rule.ID,

        &rule.ConnectedAccountID,

        &rule.Name,

        &rule.IsActive,

        &rule.TriggerConditions,

        &rule.ActionParams,

        &rule.CreatedAt,

        &rule.UpdatedAt,

    )



    if err != nil {

        if errors.Is(err, pgx.ErrNoRows) {

            return domain.AutomationRule{}, fmt.Errorf("rule not found")

        }

        return domain.AutomationRule{}, fmt.Errorf("db scan error: %w", err)

    }



    return rule, nil

}



// GetRulesForAccount ...

func (s *DBStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {

    query := `

    SELECT id, connected_account_id, name, is_active,

           trigger_conditions, action_params, created_at, updated_at

    FROM automation_rules

    WHERE connected_account_id = $1

    ORDER BY created_at DESC;

    `



    rows, err := s.pool.Query(ctx, query, accountID)

    if err != nil {

        return nil, fmt.Errorf("db query error: %w", err)

    }

    defer rows.Close()



    var rules []domain.AutomationRule

    for rows.Next() {

        var rule domain.AutomationRule

        err := rows.Scan(

            &rule.ID,

            &rule.ConnectedAccountID,

            &rule.Name,

            &rule.IsActive,

            &rule.TriggerConditions,

            &rule.ActionParams,

            &rule.CreatedAt,

            &rule.UpdatedAt,

        )

        if err != nil {

            return nil, fmt.Errorf("db row scan error: %w", err)

        }

        rules = append(rules, rule)

    }



    if err := rows.Err(); err != nil {

        return nil, fmt.Errorf("db rows error: %w", err)

    }



    return rules, nil

}



// UpdateRuleParams definieert de parameters voor het bijwerken van een regel.

type UpdateRuleParams struct {

    RuleID            uuid.UUID

    Name              string

    TriggerConditions json.RawMessage

    ActionParams      json.RawMessage

}



// UpdateRule werkt een bestaande regel bij.

func (s *DBStore) UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error) {

    query := `

    UPDATE automation_rules

    SET name = $1, trigger_conditions = $2, action_params = $3, updated_at = now()

    WHERE id = $4

    RETURNING id, connected_account_id, name, is_active, trigger_conditions, action_params, created_at, updated_at;

    `

    row := s.pool.QueryRow(ctx, query,

        arg.Name,

        arg.TriggerConditions,

        arg.ActionParams,

        arg.RuleID,

    )



    var rule domain.AutomationRule

    err := row.Scan(

        &rule.ID,

        &rule.ConnectedAccountID,

        &rule.Name,

        &rule.IsActive,

        &rule.TriggerConditions,

        &rule.ActionParams,

        &rule.CreatedAt,

        &rule.UpdatedAt,

    )



    if err != nil {

        return domain.AutomationRule{}, fmt.Errorf("db scan error: %w", err)

    }



    return rule, nil

}



// ToggleRuleStatus zet de 'is_active' boolean van een regel om.

func (s *DBStore) ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {

    query := `

    UPDATE automation_rules

    SET is_active = NOT is_active, updated_at = now()

    WHERE id = $1

    RETURNING id, connected_account_id, name, is_active, trigger_conditions, action_params, created_at, updated_at;

    `

    row := s.pool.QueryRow(ctx, query, ruleID)



    var rule domain.AutomationRule

    err := row.Scan(

        &rule.ID,

        &rule.ConnectedAccountID,

        &rule.Name,

        &rule.IsActive,

        &rule.TriggerConditions,

        &rule.ActionParams,

        &rule.CreatedAt,

        &rule.UpdatedAt,

    )



    if err != nil {

        return domain.AutomationRule{}, fmt.Errorf("db scan error: %w", err)

    }



    return rule, nil

}



// VerifyRuleOwnership controleert of een gebruiker de eigenaar is van de regel (via het account).

func (s *DBStore) VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error {

    query := `

       SELECT 1

       FROM automation_rules r

       JOIN connected_accounts ca ON r.connected_account_id = ca.id

       WHERE r.id = $1 AND ca.user_id = $2

       LIMIT 1;

       `

    var exists int

    err := s.pool.QueryRow(ctx, query, ruleID, userID).Scan(&exists)



    if err != nil {

        if errors.Is(err, pgx.ErrNoRows) {

            return fmt.Errorf("forbidden: rule not found or does not belong to user")

        }

        return fmt.Errorf("db query error: %w", err)

    }



    return nil

}



// DeleteRule verwijdert een specifieke regel uit de database.

func (s *DBStore) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {

    query := `

       DELETE FROM automation_rules

       WHERE id = $1;

       `



    cmdTag, err := s.pool.Exec(ctx, query, ruleID)

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }



    if cmdTag.RowsAffected() == 0 {

        return fmt.Errorf("no rule found with ID %s to delete", ruleID)

    }



    return nil

}



// --- LOG FUNCTIES ---



// UpdateAccountStatus

func (s *DBStore) UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {

    query := `

    UPDATE connected_accounts

    SET status = $1, updated_at = now()

    WHERE id = $2;

    `

    _, err := s.pool.Exec(ctx, query, status, id)

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }

    return nil

}



// CreateLogParams

type CreateLogParams struct {

    ConnectedAccountID uuid.UUID

    RuleID             uuid.UUID

    Status             domain.AutomationLogStatus

    TriggerDetails     json.RawMessage // []byte

    ActionDetails      json.RawMessage // []byte

    ErrorMessage       string

}



// CreateAutomationLog

func (s *DBStore) CreateAutomationLog(ctx context.Context, arg CreateLogParams) error {

    query := `

    INSERT INTO automation_logs (

        connected_account_id, rule_id, status, trigger_details, action_details, error_message

    ) VALUES ($1, $2, $3, $4, $5, $6);

    `

    _, err := s.pool.Exec(ctx, query,

        arg.ConnectedAccountID,

        arg.RuleID,

        arg.Status,

        arg.TriggerDetails,

        arg.ActionDetails,

        arg.ErrorMessage,

    )

    if err != nil {

        return fmt.Errorf("db exec error: %w", err)

    }

    return nil

}



// HasLogForTrigger

func (s *DBStore) HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error) {

    query := `

    SELECT 1

    FROM automation_logs

    WHERE rule_id = $1

      AND status = 'success'

      AND trigger_details->>'google_event_id' = $2

    LIMIT 1;

    `

    var exists int

    err := s.pool.QueryRow(ctx, query, ruleID, triggerEventID).Scan(&exists)



    if err != nil {

        if errors.Is(err, pgx.ErrNoRows) {

            return false, nil // Geen log gevonden, dit is geen error

        }

        return false, fmt.Errorf("db query error: %w", err) // Een échte error

    }



    return true, nil // Gevonden

}



// GetLogsForAccount haalt de meest recente logs op voor een account.

func (s *DBStore) GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error) {

    query := `

       SELECT id, connected_account_id, rule_id, timestamp, status,

              trigger_details, action_details, error_message

       FROM automation_logs

       WHERE connected_account_id = $1

       ORDER BY timestamp DESC

       LIMIT $2;

       `



    rows, err := s.pool.Query(ctx, query, accountID, limit)

    if err != nil {

        return nil, fmt.Errorf("db query error: %w", err)

    }

    defer rows.Close()



    var logs []domain.AutomationLog

    for rows.Next() {

        var log domain.AutomationLog

        err := rows.Scan(

            &log.ID,

            &log.ConnectedAccountID,

            &log.RuleID,

            &log.Timestamp,

            &log.Status,

            &log.TriggerDetails,

            &log.ActionDetails,

            &log.ErrorMessage,

        )

        if err != nil {

            return nil, fmt.Errorf("db row scan error: %w", err)

        }

        logs = append(logs, log)

    }



    if err := rows.Err(); err != nil {

        return nil, fmt.Errorf("db rows error: %w", err)

    }



    return logs, nil

}



// --- GECENTRALISEERDE TOKEN LOGICA ---



// getDecryptedToken is een helper om de db struct om te zetten naar een oauth2.Token

func (s *DBStore) getDecryptedToken(acc domain.ConnectedAccount) (*oauth2.Token, error) {

    plaintextAccessToken, err := crypto.Decrypt(acc.AccessToken)

    if err != nil {

        return nil, fmt.Errorf("could not decrypt access token: %w", err)

    }



    var plaintextRefreshToken []byte

    if len(acc.RefreshToken) > 0 {

        plaintextRefreshToken, err = crypto.Decrypt(acc.RefreshToken)

        if err != nil {

            return nil, fmt.Errorf("could not decrypt refresh token: %w", err)

        }

    }



    return &oauth2.Token{

        AccessToken:  string(plaintextAccessToken),

        RefreshToken: string(plaintextRefreshToken),

        Expiry:       acc.TokenExpiry,

        TokenType:    "Bearer",

    }, nil

}



// GetValidTokenForAccount is de centrale functie die een token ophaalt,

// en indien nodig ververst en opslaat.

func (s *DBStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {

    // 1. Haal account op uit DB

    acc, err := s.GetConnectedAccountByID(ctx, accountID)

    if err != nil {

        return nil, fmt.Errorf("kon account niet ophalen: %w", err)

    }



    // 2. Decrypt het token

    token, err := s.getDecryptedToken(acc)

    if err != nil {

        return nil, fmt.Errorf("kon token niet decrypten: %w", err)

    }



    // 3. Controleer of het (bijna) verlopen is

    if token.Valid() {

        return token, nil // Token is prima

    }



    // 4. Token is verlopen, ververs het

    log.Printf("[Store] Token for account %s (User %s) is expired. Refreshing...", acc.ID, acc.UserID)



    ts := s.googleOAuthConfig.TokenSource(ctx, token)

    newToken, err := ts.Token()

    if err != nil {

        // Vang 'invalid_grant'

        if strings.Contains(err.Error(), "invalid_grant") {

            log.Printf("[Store] FATAL: Access for account %s has been revoked. Setting status to 'revoked'.", acc.ID)

            if err := s.UpdateAccountStatus(ctx, acc.ID, domain.StatusRevoked); err != nil {

                log.Printf("[Store] ERROR: Failed to update status for revoked account %s: %v", acc.ID, err)

            }

            return nil, ErrTokenRevoked // Gooi specifieke error

        }

        return nil, fmt.Errorf("could not refresh token: %w", err)

    }



    // 5. Sla het nieuwe token op

    // Als we GEEN nieuwe refresh token krijgen, hergebruik dan de oude

    if newToken.RefreshToken == "" {

        newToken.RefreshToken = token.RefreshToken

    }



    err = s.UpdateAccountTokens(ctx, UpdateAccountTokensParams{

        AccountID:       acc.ID,

        NewAccessToken:  newToken.AccessToken,

        NewRefreshToken: newToken.RefreshToken, // Zorg dat we de nieuwe refresh token opslaan

        NewTokenExpiry:  newToken.Expiry,

    })

    if err != nil {

        return nil, fmt.Errorf("kon ververst token niet opslaan: %w", err)

    }



    log.Printf("[Store] Token for account %s successfully refreshed and saved.", acc.ID)



    // 6. Geef het nieuwe, geldige token terug

    return newToken, nil

}



// UpdateConnectedAccountToken update access/refresh token

func (s *DBStore) UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error {

    _, err := s.pool.Exec(ctx, `

        UPDATE connected_accounts

        SET access_token = $1, refresh_token = $2, token_expiry = $3, updated_at = now()

        WHERE id = $4

    `, params.AccessToken, params.RefreshToken, params.TokenExpiry, params.ID)

    return err

}









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\worker

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\internal\worker\worker.go

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









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\logs

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\.env

#---------------------------------------------------

# 1. APPLICATIE CONFIGURATIE

#---------------------------------------------------

APP_ENV=development



API_PORT=8080



#---------------------------------------------------

# 1.5. DATABASE MIGRATIONS

#---------------------------------------------------

RUN_MIGRATIONS=true



#---------------------------------------------------

# 2. DATABASE (POSTGRES)

#---------------------------------------------------

POSTGRES_USER=postgres

POSTGRES_PASSWORD=Bootje12

POSTGRES_DB=agenda_automator

POSTGRES_HOST=localhost

POSTGRES_PORT=5433



DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"



#---------------------------------------------------

# 4. BEVEILIGING & ENCRYPTIE

#---------------------------------------------------

ENCRYPTION_KEY="IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea"



#---------------------------------------------------

# 5. OAUTH CLIENTS (Google)

#---------------------------------------------------

CLIENT_BASE_URL="http://localhost:3000"



OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/google/callback"



GOOGLE_OAUTH_CLIENT_ID="273644756085-9tjakd3cvbkgkct2ttubpv8r9mef3jeh.apps.googleusercontent.com"

GOOGLE_OAUTH_CLIENT_SECRET="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"



# NIEUW: Voor dynamic CORS (GECORRIGEERD)

ALLOWED_ORIGINS=http://localhost:3000,https://prod.com



#---------------------------------------------------

# 6. AUTHENTICATIE (JWT) - (NIEUWE SECTIE)

#---------------------------------------------------

# Genereer een sterke, willekeurige 32-byte sleutel

JWT_SECRET_KEY="een-andere-zeer-sterke-geheime-sleutel"







C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\.env.example

//---------------------------------------------------

// 1. APPLICATIE CONFIGURATIE

//---------------------------------------------------

APP_ENV=development



API_PORT=8080



//---------------------------------------------------

// 2. DATABASE (POSTGRES)

//---------------------------------------------------

POSTGRES_USER=postgres

POSTGRES_PASSWORD=Bootje12

POSTGRES_DB=agenda_automator

POSTGRES_HOST=localhost

POSTGRES_PORT=5433



DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"



//---------------------------------------------------

// 3. CACHE (REDIS)

//---------------------------------------------------

// Verwijderd, want niet gebruikt



//---------------------------------------------------

// 4. BEVEILIGING & ENCRYPTIE

//---------------------------------------------------

ENCRYPTION_KEY="IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea"



//---------------------------------------------------

// 5. OAUTH CLIENTS (Google)

//---------------------------------------------------

CLIENT_BASE_URL="http://localhost:3000"



OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/google/callback"



GOOGLE_OAUTH_CLIENT_ID="YOUR-CLIENT-ID-HERE.apps.googleusercontent.com"

GOOGLE_OAUTH_CLIENT_SECRET="YOUR-CLIENT-SECRET-HERE"



// NIEUW: Voor dynamic CORS

ALLOWED_ORIGINS=http://localhost:3000,https://prod.com







C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\.gitignore

# Environment variables

.env



# Binaries for programs and plugins

*.exe

*.exe~

*.dll

*.so

*.dylib



# Test binary, built with `go test -c`

*.test



# Output of the go coverage tool

*.out



# Go workspace file

go.work



# IDE files

.vscode/

.idea/

*.swp

*.swo



# OS files

.DS_Store

Thumbs.db



# Logs

*.log



# Database files (for local development)

*.db

*.sqlite



# Temporary files

*.tmp

*.temp







C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\agenda-automator-api.exe

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\docker-compose.yml

version: '3.8'



services:

  db:

    image: postgres:16-alpine

    container_name: agenda_automator_db

    restart: unless-stopped

    

    environment:

      POSTGRES_USER: postgres

      POSTGRES_PASSWORD: Bootje12

      POSTGRES_DB: agenda_automator

      

    ports:

      - "5433:5432"

    volumes:

      - pgdata:/var/lib/postgresql/data

      

    healthcheck:

      test: ["CMD-SHELL", "psql -U postgres -d agenda_automator -c 'SELECT 1'"]

      interval: 10s

      timeout: 5s

      retries: 5

      start_period: 15s



  app:

    build: .

    container_name: agenda_automator_app

    restart: unless-stopped



    ports:

      - "8080:8080"



    depends_on:

      db:

        condition: service_healthy



    environment:

      - DATABASE_URL=postgres://postgres:Bootje12@db:5432/agenda_automator?sslmode=disable

      - APP_ENV=development

      - API_PORT=8080

      - ENCRYPTION_KEY=IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea

      - CLIENT_BASE_URL=http://localhost:3000

      - OAUTH_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback

      - GOOGLE_OAUTH_CLIENT_ID=${GOOGLE_OAUTH_CLIENT_ID}

      - GOOGLE_OAUTH_CLIENT_SECRET=${GOOGLE_OAUTH_CLIENT_SECRET}

      - RUN_MIGRATIONS=true

      - LOG_MAX_SIZE=10MB

      - ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

      # HIER IS DE TOEVOEGING:

      - JWT_SECRET_KEY=${JWT_SECRET_KEY}



volumes:

  pgdata:







C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\Dockerfile

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







C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\go.mod

module agenda-automator-api



go 1.24.0



toolchain go1.24.10



require (

    github.com/go-chi/chi/v5 v5.2.3

    github.com/go-chi/cors v1.2.2

    github.com/golang-jwt/jwt/v5 v5.3.0

    github.com/google/uuid v1.6.0

    github.com/jackc/pgx/v5 v5.7.6

    github.com/joho/godotenv v1.5.1

    golang.org/x/oauth2 v0.33.0

    google.golang.org/api v0.256.0

)



require (

    cloud.google.com/go/auth v0.17.0 // indirect

    cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect

    cloud.google.com/go/compute/metadata v0.9.0 // indirect

    github.com/felixge/httpsnoop v1.0.4 // indirect

    github.com/go-logr/logr v1.4.3 // indirect

    github.com/go-logr/stdr v1.2.2 // indirect

    github.com/google/s2a-go v0.1.9 // indirect

    github.com/googleapis/enterprise-certificate-proxy v0.3.7 // indirect

    github.com/googleapis/gax-go/v2 v2.15.0 // indirect

    github.com/jackc/pgpassfile v1.0.0 // indirect

    github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect

    github.com/jackc/puddle/v2 v2.2.2 // indirect

    go.opentelemetry.io/auto/sdk v1.1.0 // indirect

    go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect

    go.opentelemetry.io/otel v1.37.0 // indirect

    go.opentelemetry.io/otel/metric v1.37.0 // indirect

    go.opentelemetry.io/otel/trace v1.37.0 // indirect

    golang.org/x/crypto v0.43.0 // indirect

    golang.org/x/net v0.46.0 // indirect

    golang.org/x/sync v0.18.0 // indirect

    golang.org/x/sys v0.37.0 // indirect

    golang.org/x/text v0.30.0 // indirect

    google.golang.org/genproto/googleapis/rpc v0.0.0-20251103181224-f26f9409b101 // indirect

    google.golang.org/grpc v1.76.0 // indirect

    google.golang.org/protobuf v1.36.10 // indirect

)









C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\go.sum

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\main.exe

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\migrate.exe

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\README.md

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\server.exe

C:\Users\jeffrey\Desktop\Githubmains\AgendaTool FrontBackend\Backend\Jeffrey-s-Agenda-Tool BACKEND\Volledige Codebase Backend.md



