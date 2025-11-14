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
