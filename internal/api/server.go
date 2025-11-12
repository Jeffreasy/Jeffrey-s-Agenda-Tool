package api

import (
	"agenda-automator-api/internal/store"
	"net/http"
	"os"      // NIEUW voor env
	"strings" // NIEUW voor split
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/oauth2"
)

// Server is de hoofdstruct voor je API
type Server struct {
	store             store.Storer
	Router            *chi.Mux
	googleOAuthConfig *oauth2.Config
}

// NewServer (AANGEPAST) - accepteert nu de config
func NewServer(s store.Storer, oauthCfg *oauth2.Config) *Server {
	r := chi.NewRouter()

	// --- Middleware ---
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS dynamic vanuit .env (bijv. ALLOWED_ORIGINS=http://localhost:3000,https://prod.com)
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

// registerRoutes koppelt URL-paden aan handler-functies
func (s *Server) registerRoutes() {
	// We groeperen alle v1 routes onder /api/v1
	s.Router.Route("/api/v1", func(r chi.Router) {

		// --- NIEUWE AUTH ROUTES ---
		r.Get("/auth/google/login", s.handleGoogleLogin())
		r.Get("/auth/google/callback", s.handleGoogleCallback())
		// --- EINDE NIEUWE ROUTES ---

		// /api/v1/health
		r.Get("/health", s.handleHealthCheck())

		// /api/v1/users
		r.Post("/users", s.handleCreateUser())

		// NIEUW: /api/v1/rules
		r.Post("/rules", s.handleCreateAutomationRule())
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
