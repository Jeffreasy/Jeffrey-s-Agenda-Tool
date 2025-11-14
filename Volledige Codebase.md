# Inhoudsopgave

- [Inhoudsopgave](#inhoudsopgave)
	- [cmd/server/main.go](#cmdservermaingo)
	- [db/migrations/000001\_initial\_schema.down.sql](#dbmigrations000001_initial_schemadownsql)
	- [db/migrations/000001\_initial\_schema.up.sql](#dbmigrations000001_initial_schemaupsql)
	- [internal/api/handlers.go](#internalapihandlersgo)
	- [internal/api/json.go](#internalapijsongo)
	- [internal/api/server.go](#internalapiservergo)
	- [internal/crypto/crypto.go](#internalcryptocryptogo)
	- [internal/database/database.go](#internaldatabasedatabasego)
	- [internal/domain/models.go](#internaldomainmodelsgo)
	- [internal/store/store.go](#internalstorestorego)
	- [internal/worker/worker.go](#internalworkerworkergo)
- [1. APPLICATIE CONFIGURATIE](#1-applicatie-configuratie)
- [1.5. DATABASE MIGRATIONS](#15-database-migrations)
- [2. DATABASE (POSTGRES)](#2-database-postgres)
- [4. BEVEILIGING \& ENCRYPTIE](#4-beveiliging--encryptie)
- [5. OAUTH CLIENTS (Google)](#5-oauth-clients-google)
- [NIEUW: Voor dynamic CORS](#nieuw-voor-dynamic-cors)
- [6. AUTHENTICATIE (JWT) - (NIEUWE SECTIE)](#6-authenticatie-jwt---nieuwe-sectie)
- [Genereer een sterke, willekeurige 32-byte sleutel](#genereer-een-sterke-willekeurige-32-byte-sleutel)
	- [.env.example](#envexample)
	- [.gitignore](#gitignore)
- [Environment variables](#environment-variables)
- [Binaries for programs and plugins](#binaries-for-programs-and-plugins)
- [Test binary, built with `go test -c`](#test-binary-built-with-go-test--c)
- [Output of the go coverage tool](#output-of-the-go-coverage-tool)
- [Go workspace file](#go-workspace-file)
- [IDE files](#ide-files)
- [OS files](#os-files)
- [Logs](#logs)
- [Database files (for local development)](#database-files-for-local-development)
- [Temporary files](#temporary-files)
	- [docker-compose.yml](#docker-composeyml)
	- [go.mod](#gomod)
	- [go.sum](#gosum)
	- [README.md](#readmemd)

## cmd/server/main.go
package main

import (
	"log"
	"net/http"
	"os"

	// NIEUW voor CORS
	// Interne packages
	"agenda-automator-api/internal/api"
	"agenda-automator-api/internal/database"
	"agenda-automator-api/internal/store"
	"agenda-automator-api/internal/worker"

	// Externe packages
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	// 1. Laad configuratie (.env)
	_ = godotenv.Load()

	// 2. Maak verbinding met de Database
	pool, err := database.ConnectDB()
	if err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}
	defer pool.Close()

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

	// 4. Initialiseer de 'Store' Laag
	dbStore := store.NewStore(pool)

	// 5. Initialiseer de Worker (geef de config mee)
	appWorker, err := worker.NewWorker(dbStore, googleOAuthConfig)
	if err != nil {
		log.Fatalf("Could not initialize worker: %v", err)
	}

	// 6. Start de Worker in de achtergrond
	appWorker.Start()

	// 7. Initialiseer de API Server (geef de config mee)
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


## db/migrations/000001_initial_schema.down.sql
-- Rollback initial schema migration
-- Add DROP statements in reverse order here

-- Example (replace with your actual rollback statements):
-- DROP TABLE IF EXISTS automation_logs;
-- DROP TABLE IF EXISTS automation_rules;
-- DROP TABLE IF EXISTS connected_accounts;
-- DROP TABLE IF EXISTS users;

-- DROP TYPE IF EXISTS automation_status;
-- DROP TYPE IF EXISTS account_status;
-- DROP TYPE IF EXISTS provider_type;

## db/migrations/000001_initial_schema.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE provider_type AS ENUM (
    'google',
    'microsoft'
);

CREATE TYPE account_status AS ENUM (
    'active',
    'revoked',
    'error',
    'paused'
);

CREATE TYPE automation_log_status AS ENUM (
    'pending',
    'success',
    'failure',
    'skipped'
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connected_accounts (
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
CREATE INDEX idx_connected_accounts_user_id ON connected_accounts(user_id);

CREATE TABLE automation_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    name text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    trigger_conditions jsonb NOT NULL,
    action_params jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_automation_rules_account_id ON automation_rules(connected_account_id) WHERE is_active = true;

CREATE TABLE automation_logs (
    id bigserial PRIMARY KEY,
    connected_account_id uuid NOT NULL REFERENCES connected_accounts(id) ON DELETE CASCADE,
    rule_id uuid REFERENCES automation_rules(id) ON DELETE SET NULL,
    timestamp timestamptz NOT NULL DEFAULT now(),
    status automation_log_status NOT NULL,
    trigger_details jsonb,
    action_details jsonb,
    error_message text
);
CREATE INDEX idx_automation_logs_account_id_timestamp ON automation_logs(connected_account_id, timestamp DESC);
CREATE INDEX idx_automation_logs_rule_id ON automation_logs(rule_id, timestamp DESC);

## internal/api/handlers.go
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

	"github.com/go-chi/chi/v5" // NIEUWE IMPORT
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// --- AUTH HANDLERS ---

const oauthStateCookieName = "oauthstate"

// NIEUWE HELPER: Haalt de user ID op die door de middleware in de context is gezet
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

// --- CreateUser (Bestaande code) ---

type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) handleCreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Email == "" {
			WriteJSONError(w, http.StatusBadRequest, "Email is required")
			return
		}

		user, err := s.store.CreateUser(r.Context(), req.Email, req.Name)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Could not create user")
			return
		}

		WriteJSON(w, http.StatusCreated, user)
	}
}

// --- handleCreateAutomationRule (Bestaande code) ---

type CreateAutomationRuleRequest struct {
	ConnectedAccountID uuid.UUID       `json:"connected_account_id"`
	Name               string          `json:"name"`
	TriggerConditions  json.RawMessage `json:"trigger_conditions"`
	ActionParams       json.RawMessage `json:"action_params"`
}

func (s *Server) handleCreateAutomationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker (uit de context)
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Decodeer de request body
		var req CreateAutomationRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// 3. (KRITIEK) Controleer of deze gebruiker wel de eigenaar is van het account
		if err := s.store.VerifyAccountOwnership(r.Context(), req.ConnectedAccountID, userID); err != nil {
			log.Printf("Forbidden access attempt: User %s tried to access account %s", userID, req.ConnectedAccountID)
			WriteJSONError(w, http.StatusForbidden, "Je hebt geen toegang tot dit account")
			return
		}

		// 4. Ga door met het aanmaken van de regel (nu veilig)
		params := store.CreateAutomationRuleParams{
			ConnectedAccountID: req.ConnectedAccountID, // We weten nu dat dit ID veilig is
			Name:               req.Name,
			TriggerConditions:  req.TriggerConditions,
			ActionParams:       req.ActionParams,
		}

		rule, err := s.store.CreateAutomationRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon regel niet aanmaken")
			return
		}

		WriteJSON(w, http.StatusCreated, rule)
	}
}

// --- NIEUWE READ HANDLERS ---

// handleGetConnectedAccounts haalt alle accounts op voor de ingelogde gebruiker
func (s *Server) handleGetConnectedAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Haal de accounts op uit de store
		accounts, err := s.store.GetAccountsForUser(r.Context(), userID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon accounts niet ophalen")
			return
		}

		// 3. (BELANGRIJK) Verberg de versleutelde tokens voordat we ze terugsturen.
		// We willen nooit access/refresh tokens (zelfs versleuteld) naar de client sturen.
		type PublicAccount struct {
			ID             uuid.UUID           `json:"id"`
			UserID         uuid.UUID           `json:"user_id"`
			Provider       domain.ProviderType `json:"provider"`
			Email          string              `json:"email"`
			Status         domain.AccountStatus`json:"status"`
			ProviderUserID string              `json:"provider_user_id"`
			CreatedAt      time.Time           `json:"created_at"`
		}

		publicAccounts := make([]PublicAccount, len(accounts))
		for i, acc := range accounts {
			publicAccounts[i] = PublicAccount{
				ID:             acc.ID,
				UserID:         acc.UserID,
				Provider:       acc.Provider,
				Email:          acc.Email,
				Status:         acc.Status,
				ProviderUserID: acc.ProviderUserID,
				CreatedAt:      acc.CreatedAt,
			}
		}

		WriteJSON(w, http.StatusOK, publicAccounts)
	}
}

// handleGetAutomationRules haalt alle regels op voor een specifiek, eigendom
func (s *Server) handleGetAutomationRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Haal de 'accountID' op uit de URL path (e.g. /accounts/uuid-hier/rules)
		accountIDStr := chi.URLParam(r, "accountID")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID formaat")
			return
		}

		// 3. (KRITIEK) Controleer eigendom
		if err := s.store.VerifyAccountOwnership(r.Context(), accountID, userID); err != nil {
			log.Printf("Forbidden access attempt: User %s tried to access rules for account %s", userID, accountID)
			WriteJSONError(w, http.StatusForbidden, "Je hebt geen toegang tot dit account")
			return
		}

		// 4. Haal de regels op (nu veilig)
		rules, err := s.store.GetRulesForAccount(r.Context(), accountID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon regels niet ophalen")
			return
		}

		// De 'rules' struct bevat geen geheimen, dus we kunnen het direct teruggeven.
		WriteJSON(w, http.StatusOK, rules)
	}
}






## internal/api/json.go
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



## internal/api/server.go
package api

import (
	"agenda-automator-api/internal/store"
	"context" // NIEUWE IMPORT
	"fmt"     // NIEUWE IMPORT
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5" // NIEUWE IMPORT
	"github.com/google/uuid"       // NIEUWE IMPORT
	"golang.org/x/oauth2"
)

// Server is de hoofdstruct voor je API
type Server struct {
	store             store.Storer
	Router            *chi.Mux
	googleOAuthConfig *oauth2.Config
}

// NIEUW: Context key voor de user ID
type contextKey string

const userContextKey contextKey = "userID"

// NewServer (AANGEPAST) - accepteert nu de config
func NewServer(s store.Storer, oauthCfg *oauth2.Config) *Server {
	r := chi.NewRouter()

	// --- Middleware ---
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS dynamic vanuit .env
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000"}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	server := &Server{
		store:             s,
		Router:            r,
		googleOAuthConfig: oauthCfg,
	}

	// Koppel de routes aan functies
	server.registerRoutes()

	return server
}

// registerRoutes (AANGEPAST om routes te beveiligen)
func (s *Server) registerRoutes() {
	// We groeperen alle v1 routes onder /api/v1
	s.Router.Route("/api/v1", func(r chi.Router) {

		// --- Publieke Routes (geen auth nodig) ---
		r.Get("/health", s.handleHealthCheck())
		r.Get("/auth/google/login", s.handleGoogleLogin())
		r.Get("/auth/google/callback", s.handleGoogleCallback())
		r.Post("/users", s.handleCreateUser()) // (Waarschijnlijk overbodig, maar laten staan)

		// --- Beveiligde Routes (wel auth nodig) ---
		// Alle routes binnen deze group vereisen een geldig JWT token
		r.Group(func(r chi.Router) {
			r.Use(s.jwtAuthMiddleware())

			// --- Accounts Routes ---
			// GET /api/v1/accounts (NIEUW)
			r.Get("/accounts", s.handleGetConnectedAccounts())

			// --- Rules Routes ---
			// POST /api/v1/rules (Was al beveiligd)
			r.Post("/rules", s.handleCreateAutomationRule())

			// GET /api/v1/accounts/{accountID}/rules (NIEUW)
			r.Get("/accounts/{accountID}/rules", s.handleGetAutomationRules())
		})
	})
}

// handleHealthCheck (Bestaande code)
func (s *Server) handleHealthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	}
}

// --- NIEUWE AUTHENTICATIE MIDDLEWARE ---

func (s *Server) jwtAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))
			if len(jwtKey) == 0 {
				WriteJSONError(w, http.StatusInternalServerError, "JWT_SECRET_KEY is niet geconfigureerd")
				return
			}

			// 1. Haal de token-string op uit de "Authorization" header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteJSONError(w, http.StatusUnauthorized, "Geen Authorization header")
				return
			}

			// 2. Controleer of het "Bearer " prefix heeft
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				WriteJSONError(w, http.StatusUnauthorized, "Ongeldig token formaat (mist 'Bearer ' prefix)")
				return
			}

			// 3. Valideer het token
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Controleer de signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("onverwachte signing method: %v", token.Header["alg"])
				}
				return jwtKey, nil
			})

			if err != nil {
				WriteJSONError(w, http.StatusUnauthorized, fmt.Sprintf("Ongeldig token: %v", err))
				return
			}

			// 4. Haal de claims (data) en user_id eruit
			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				userIDStr, ok := claims["user_id"].(string)
				if !ok {
					WriteJSONError(w, http.StatusUnauthorized, "Ongeldige token claims (geen user_id)")
					return
				}

				userID, err := uuid.Parse(userIDStr)
				if err != nil {
					WriteJSONError(w, http.StatusUnauthorized, "Ongeldige user_id in token")
					return
				}

				// 5. Voeg de user_id toe aan de context voor de volgende handlers
				ctx := context.WithValue(r.Context(), userContextKey, userID)
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				WriteJSONError(w, http.StatusUnauthorized, "Ongeldig token")
			}
		})
	}
}




## internal/crypto/crypto.go
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



## internal/database/database.go
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



## internal/domain/models.go
package domain

import (
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

type AutomationLogStatus string // Aangepast voor consistentie

const (
	LogPending AutomationLogStatus = "pending"
	LogSuccess AutomationLogStatus = "success"
	LogFailure AutomationLogStatus = "failure"
	LogSkipped AutomationLogStatus = "skipped"
)

// --- Tabel Structs ---
type User struct {
	ID        uuid.UUID `db:"id"`
	Email     string    `db:"email"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type ConnectedAccount struct {
	ID             uuid.UUID     `db:"id"`
	UserID         uuid.UUID     `db:"user_id"`
	Provider       ProviderType  `db:"provider"`
	Email          string        `db:"email"`
	ProviderUserID string        `db:"provider_user_id"`
	AccessToken    []byte        `db:"access_token"` // Blijft []byte voor encryptie
	RefreshToken   []byte        `db:"refresh_token"`
	TokenExpiry    time.Time     `db:"token_expiry"`
	Scopes         []string      `db:"scopes"` // pgx kan []string naar text[] mappen
	Status         AccountStatus `db:"status"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
	LastChecked    time.Time     `db:"last_checked"` // NIEUW
}

type AutomationRule struct {
	ID                 uuid.UUID `db:"id"`
	ConnectedAccountID uuid.UUID `db:"connected_account_id"`
	Name               string    `db:"name"`
	IsActive           bool      `db:"is_active"`
	TriggerConditions  []byte    `db:"trigger_conditions"` // JSONB as []byte
	ActionParams       []byte    `db:"action_params"`      // JSONB as []byte
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

type AutomationLog struct {
	ID                 int64               `db:"id"` // bigserial mapt naar int64
	ConnectedAccountID uuid.UUID           `db:"connected_account_id"`
	RuleID             uuid.UUID           `db:"rule_id"` // pgx kan 'NULL' UUID's aan
	Timestamp          time.Time           `db:"timestamp"`
	Status             AutomationLogStatus `db:"status"`
	TriggerDetails     []byte              `db:"trigger_details"` // JSONB as []byte
	ActionDetails      []byte              `db:"action_details"`  // JSONB as []byte
	ErrorMessage       string              `db:"error_message"`   // pgx mapt 'NULL' text naar ""
}

// Aangepaste structs voor JSONB met meer flexibiliteit voor shift automation
type TriggerConditions struct {
	SummaryEquals    string   `json:"summary_equals,omitempty"`
	SummaryContains  []string `json:"summary_contains,omitempty"`
	LocationContains []string `json:"location_contains,omitempty"`
}

type ActionParams struct {
	OffsetMinutes int    `json:"offset_minutes"` // Bijv. -60 voor 1 uur voor shift
	NewEventTitle string `json:"new_event_title"`
	DurationMin   int    `json:"duration_min"`   // Bijv. 5 voor reminder duur
}

// --- OPTIMALISATIE: Structs voor JSONB Logging ---
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



## internal/store/store.go
package store

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Storer is de interface voor al onze database-interacties.
type Storer interface {
	CreateUser(ctx context.Context, email, name string) (domain.User, error)

	UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error)
	GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error)
	GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error)
	GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) // NIEUWE FUNCTIE
	UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error
	UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error
	UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error
	VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error

	CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error)
	GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error)
	CreateAutomationLog(ctx context.Context, arg CreateLogParams) error
	HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error)
}

// DBStore implementeert de Storer interface.
type DBStore struct {
	pool *pgxpool.Pool
}

// NewStore maakt een nieuwe DBStore
func NewStore(pool *pgxpool.Pool) Storer {
	return &DBStore{
		pool: pool,
	}
}

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

// UpdateAccountTokensParams ...
type UpdateAccountTokensParams struct {
	AccountID       uuid.UUID
	NewAccessToken  string
	NewRefreshToken string
	NewTokenExpiry  time.Time
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
		return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)
	}

	return acc, nil
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

// --- NIEUWE READ FUNCTIE ---
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

// --- AUTH FUNCTIE ---
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

// GetRulesForAccount ...
func (s *DBStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	query := `
    SELECT id, connected_account_id, name, is_active,
           trigger_conditions, action_params, created_at, updated_at
    FROM automation_rules
    WHERE connected_account_id = $1 AND is_active = true;
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

// --- OPTIMALISATIE 2: ERROR HANDLING ---
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

// --- OPTIMALISATIE 1: LOGGING PARAMS ---
type CreateLogParams struct {
	ConnectedAccountID uuid.UUID
	RuleID             uuid.UUID
	Status             domain.AutomationLogStatus
	TriggerDetails     json.RawMessage // []byte
	ActionDetails      json.RawMessage // []byte
	ErrorMessage       string
}

// --- OPTIMALISATIE 1: LOGGING ---
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

// --- OPTIMALISATIE 1: EFFICIENCY CHECK ---
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




## internal/worker/worker.go
package worker

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"encoding/json"
	"errors" // NIEUW
	"fmt"
	"log"
	"strings"
	"sync" // NIEUW voor parallel
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// NIEUW: (Optimalisatie 2) Specifieke error voor revoked tokens
var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

// Worker is de struct voor onze achtergrond-processor
type Worker struct {
	store             store.Storer
	googleOAuthConfig *oauth2.Config
}

// NewWorker ...
func NewWorker(s store.Storer, oauthCfg *oauth2.Config) (*Worker, error) {
	if oauthCfg == nil {
		return nil, fmt.Errorf("oauth config mag niet nil zijn")
	}

	return &Worker{
		store:             s,
		googleOAuthConfig: oauthCfg,
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

// processAccount (AANGEPAST met Optimalisatie 2)
func (w *Worker) processAccount(ctx context.Context, acc domain.ConnectedAccount) {
	// Maak een 'token' object van de data in de DB
	token, err := w.getTokenForAccount(acc)
	if err != nil {
		log.Printf("[Worker] ERROR: Kon token niet voorbereiden voor account %s: %v", acc.ID, err)
		return
	}

	// Controleer of het token (bijna) verlopen is
	if !token.Valid() {
		log.Printf("[Worker] Token for account %s (User %s) is expired. Refreshing...", acc.ID, acc.UserID)

		newToken, err := w.refreshAccountToken(ctx, token)
		if err != nil {
			// --- OPTIMALISATIE 2: ERROR HANDLING ---
			if errors.Is(err, ErrTokenRevoked) {
				log.Printf("[Worker] FATAL: Access for account %s has been revoked. Setting status to 'revoked'.", acc.ID)
				// Zet account op 'revoked' in DB zodat we het niet opnieuw proberen
				if err := w.store.UpdateAccountStatus(ctx, acc.ID, domain.StatusRevoked); err != nil {
					log.Printf("[Worker] ERROR: Failed to update status for revoked account %s: %v", acc.ID, err)
				}
			} else {
				// Andere, tijdelijke refresh error
				log.Printf("[Worker] ERROR refreshing account %s: %v", acc.ID, err)
			}
			return // Stop verwerking voor dit account
			// --- EINDE OPTIMALISATIE 2 ---
		}

		err = w.store.UpdateAccountTokens(ctx, store.UpdateAccountTokensParams{
			AccountID:       acc.ID,
			NewAccessToken:  newToken.AccessToken,
			NewRefreshToken: newToken.RefreshToken, // Zorg dat we de nieuwe refresh token opslaan
			NewTokenExpiry:  newToken.Expiry,
		})
		if err != nil {
			log.Printf("[Worker] ERROR updating refreshed token for account %s: %v", acc.ID, err)
			return
		}

		token = newToken
	}

	// Process calendar
	log.Printf("[Worker] Token for account %s is valid. Processing calendar...", acc.ID)
	if err := w.processCalendarEvents(ctx, acc, token); err != nil {
		log.Printf("[Worker] ERROR processing calendar for account %s: %v", acc.ID, err)
	}

	// Update last_checked
	if err := w.store.UpdateAccountLastChecked(ctx, acc.ID); err != nil {
		log.Printf("[Worker] ERROR updating last_checked for account %s: %v", acc.ID, err)
	}
}

// getTokenForAccount haalt de versleutelde tokens op en maakt een *oauth2.Token (decrypt both)
func (w *Worker) getTokenForAccount(acc domain.ConnectedAccount) (*oauth2.Token, error) {
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

// refreshAccountToken (AANGEPAST met Optimalisatie 2)
func (w *Worker) refreshAccountToken(ctx context.Context, expiredToken *oauth2.Token) (*oauth2.Token, error) {
	ts := w.googleOAuthConfig.TokenSource(ctx, expiredToken)

	newToken, err := ts.Token()
	if err != nil {
		// --- OPTIMALISATIE 2: Vang 'invalid_grant' ---
		// Dit gebeurt als de gebruiker de toegang intrekt
		if strings.Contains(err.Error(), "invalid_grant") {
			return nil, ErrTokenRevoked
		}
		// --- EINDE OPTIMALISATIE 2 ---
		return nil, fmt.Errorf("could not refresh token: %w", err)
	}

	// Als we GEEN nieuwe refresh token krijgen, hergebruik dan de oude
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = expiredToken.RefreshToken
	}

	return newToken, nil
}

// eventExists controleert of een event met dezelfde titel al bestaat in de tijdsslot
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

// processCalendarEvents (AANGEPAST met Optimalisatie 1: Logging)
func (w *Worker) processCalendarEvents(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {

	client := w.googleOAuthConfig.Client(ctx, token)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create calendar service: %w", err)
	}

	// Aangepast: Check 3 maanden vooruit.
	tMin := time.Now().Format(time.RFC3339)
	tMax := time.Now().AddDate(0, 3, 0).Format(time.RFC3339) // 3 maanden vooruit

	log.Printf("[Worker] Fetching calendar events for %s between %s and %s", acc.Email, tMin, tMax)

	events, err := srv.Events.List("primary").
		TimeMin(tMin).
		TimeMax(tMax).
		SingleEvents(true).
		OrderBy("startTime").
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

		for _, rule := range rules {
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
			// Sla de rest over als we deze trigger event al succesvol hebben verwerkt.
			// Dit voorkomt dubbel werk en onnodige eventExists API calls.
			hasLogged, err := w.store.HasLogForTrigger(ctx, rule.ID, event.Id)
			if err != nil {
				log.Printf("[Worker] ERROR checking logs for event %s / rule %s: %v", event.Id, rule.ID, err)
				// Ga door (veilige aanname), de eventExists check vangt het wel
			}
			if hasLogged {
				// log.Printf("[Worker] SKIP: Trigger event %s already processed by rule %s.", event.Id, rule.ID)
				continue // Ga naar de volgende regel
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
			if offset == 0 { offset = -60 }
			reminderTime := startTime.Add(time.Duration(offset) * time.Minute)

			durMin := action.DurationMin
			if durMin == 0 { durMin = 5 }
			endTime := reminderTime.Add(time.Duration(durMin) * time.Minute)

			title := action.NewEventTitle

			// --- 3. CONTROLEER OP DUPLICATEN (SECUNDAIRE CHECK) ---
			if eventExists(srv, reminderTime, endTime, title) {
				log.Printf("[Worker] SKIP: Reminder event '%s' at %s already exists.", title, reminderTime)

				// --- 3.1. LOG DIT VOOR DE VOLGENDE KEER (OPTIMALISATIE 1) ---
				// We vonden geen log (stap 1.5), maar het event bestaat wel (stap 3).
				// We loggen het nu alsnog om toekomstige API-checks te voorkomen.
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
					Status:             domain.LogSkipped, // We skippen, want het bestond al
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





C:\Users\JJALa\Desktop\Github\Agenda tool Jeffrey\Jeffrey-s-Agenda-Tool\.env
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

# NIEUW: Voor dynamic CORS
ALLOWED_O_S=http://localhost:3000,https://prod.com

#---------------------------------------------------
# 6. AUTHENTICATIE (JWT) - (NIEUWE SECTIE)
#---------------------------------------------------
# Genereer een sterke, willekeurige 32-byte sleutel
JWT_SECRET_KEY="een-andere-zeer-sterke-geheime-sleutel"


## .env.example
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



## .gitignore
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



## docker-compose.yml
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

volumes:
  pgdata:



## go.mod
module agenda-automator-api

go 1.24.0

toolchain go1.24.10

require (
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/cors v1.2.2
	github.com/golang-jwt/jwt/v5 v5.2.1 // <-- NIEUWE AFHANKELIJKHEID
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




## go.sum
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




C:\Users\JJALa\Desktop\Github\Agenda tool Jeffrey\Jeffrey-s-Agenda-Tool\main.exe
## README.md
C:\Users\JJALa\Desktop\Github\Agenda tool Jeffrey\Jeffrey-s-Agenda-Tool\server.exe