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
