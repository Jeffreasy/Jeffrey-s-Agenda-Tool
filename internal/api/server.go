package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"agenda-automator-api/internal/api/account"
	"agenda-automator-api/internal/api/auth"
	"agenda-automator-api/internal/api/calendar"
	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/api/gmail"
	"agenda-automator-api/internal/api/health"
	"agenda-automator-api/internal/api/log"
	"agenda-automator-api/internal/api/rule"
	"agenda-automator-api/internal/api/user"
	"agenda-automator-api/internal/store"

	"github.com/go-chi/chi/v5" // <-- HIER ZAT DE TYPO
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// VERWIJDERD: De helper GetUserIDFromContext stond al in common.go
// (Je kunt deze laten staan als je wilt, maar het is dubbel)

type Server struct {
	Router            *chi.Mux
	Store             store.Storer
	Logger            *zap.Logger
	GoogleOAuthConfig *oauth2.Config
}

func NewServer(s store.Storer, logger *zap.Logger, oauthConfig *oauth2.Config) *Server {
	server := &Server{
		Router:            chi.NewRouter(),
		Store:             s,
		Logger:            logger,
		GoogleOAuthConfig: oauthConfig,
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
		// Health check (unprotected)
		// AANGEPAST: health.HandleHealth accepteert nu ook de logger
		r.Get("/health", health.HandleHealth(s.Logger))

		// Auth routes
		// AANGEPAST: Doorgeven s.Logger
		r.Get("/auth/google/login", auth.HandleGoogleLogin(s.GoogleOAuthConfig, s.Logger))
		r.Get("/auth/google/callback", auth.HandleGoogleCallback(s.Store, s.GoogleOAuthConfig, s.Logger))

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// User routes
			// AANGEPAST: Logger wordt nu correct doorgegeven
			r.Get("/me", user.HandleGetMe(s.Store, s.Logger))
			r.Get("/users/me", user.HandleGetMe(s.Store, s.Logger))

			// Account routes
			// AANGEPAST: Doorgeven s.Logger
			r.Get("/accounts", account.HandleGetConnectedAccounts(s.Store, s.Logger))
			r.Delete("/accounts/{accountId}", account.HandleDeleteConnectedAccount(s.Store, s.Logger))

			// Rule routes
			// AANGEPAST: Doorgeven s.Logger
			r.Post("/accounts/{accountId}/rules", rule.HandleCreateRule(s.Store, s.Logger))
			r.Get("/accounts/{accountId}/rules", rule.HandleGetRules(s.Store, s.Logger))
			r.Put("/rules/{ruleId}", rule.HandleUpdateRule(s.Store, s.Logger))
			r.Delete("/rules/{ruleId}", rule.HandleDeleteRule(s.Store, s.Logger))
			r.Put("/rules/{ruleId}/toggle", rule.HandleToggleRule(s.Store, s.Logger))

			// Log routes
			// AANGEPAST: Doorgeven s.Logger
			r.Get("/accounts/{accountId}/logs", log.HandleGetAutomationLogs(s.Store, s.Logger))

			// Calendar routes (waren al correct)
			r.Get("/accounts/{accountId}/calendars", calendar.HandleListCalendars(s.Store, s.Logger))
			r.Get("/accounts/{accountId}/calendar/events", calendar.HandleGetCalendarEvents(s.Store, s.Logger))
			r.Post("/accounts/{accountId}/calendar/events", calendar.HandleCreateEvent(s.Store, s.Logger))
			r.Put("/accounts/{accountId}/calendar/events/{eventId}", calendar.HandleUpdateEvent(s.Store, s.Logger))
			r.Delete("/accounts/{accountId}/calendar/events/{eventId}", calendar.HandleDeleteEvent(s.Store, s.Logger))
			r.Post("/calendar/aggregated-events", calendar.HandleGetAggregatedEvents(s.Store, s.Logger))

			// Gmail routes
			// AANGEPAST: Doorgeven s.Logger
			r.Get("/accounts/{accountId}/gmail/messages", gmail.HandleGetGmailMessages(s.Store, s.Logger))
			r.Post("/accounts/{accountId}/gmail/send", gmail.HandleSendGmailMessage(s.Store, s.Logger))
			r.Get("/accounts/{accountId}/gmail/labels", gmail.HandleGetGmailLabels(s.Store, s.Logger))
			r.Post("/accounts/{accountId}/gmail/drafts", gmail.HandleCreateGmailDraft(s.Store, s.Logger))
			r.Get("/accounts/{accountId}/gmail/drafts", gmail.HandleGetGmailDrafts(s.Store, s.Logger))

			// Gmail automation rules
			// AANGEPAST: Doorgeven s.Logger
			r.Post("/accounts/{accountId}/gmail/rules", gmail.HandleCreateGmailRule(s.Store, s.Logger))
			r.Get("/accounts/{accountId}/gmail/rules", gmail.HandleGetGmailRules(s.Store, s.Logger))
		})
	})
}

// authMiddleware valideert JWT en zet user ID in context
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// AANGEPAST: s.Logger meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, "Geen authenticatie header", s.Logger)
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
			// AANGEPAST: s.Logger meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, "Ongeldige token", s.Logger)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			// AANGEPAST: s.Logger meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, "Ongeldige claims", s.Logger)
			return
		}

		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			// AANGEPAST: s.Logger meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, "Geen user ID in token", s.Logger)
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			// AANGEPAST: s.Logger meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, "Ongeldig user ID", s.Logger)
			return
		}

		ctx := context.WithValue(r.Context(), common.UserContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
