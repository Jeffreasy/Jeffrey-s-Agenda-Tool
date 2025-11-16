package main

import (
	"context"
	"time"
	// _ "embed" // <-- VERWIJDERD

	"net/http"
	"os"

	// Interne packages
	"agenda-automator-api/internal/api"
	"agenda-automator-api/internal/database"
	"agenda-automator-api/internal/logger"
	"agenda-automator-api/internal/store"
	"agenda-automator-api/internal/worker"

	// <-- TOEGEVOEGD

	// Externe packages
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VERWIJDERD:
// //go:embed"../db/migrations/000001_initial_schema.up.sql"
// var migrationFile string

func main() {
	// 1. Laad configuratie (.env)
	_ = godotenv.Load()

	// 1.5. Initialiseer logger
	log, err := logger.NewLogger()
	if err != nil {
		panic("Could not initialize logger: " + err.Error()) // Can't log if logger fails
	}
	defer log.Sync()

	// 2. Maak verbinding met de Database
	pool, err := database.ConnectDB(log)
	if err != nil {
		log.Error("could not connect to the database", zap.Error(err))
		return
	}
	defer pool.Close()

	// -----------------------------------------------------
	// AANGEPAST: Stap 2.5 - Voer migraties uit
	if err = database.RunMigrations(context.Background(), pool, log); err != nil {
		log.Error("database migrations failed", zap.Error(err))
		return
	}
	// -----------------------------------------------------

	// 3. Initialiseer de Gedeelde OAuth2 Config
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	// e.g. http://localhost:8080/api/v1/auth/google/callback
	redirectURL := os.Getenv("OAUTH_REDIRECT_URL")

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Error("Google OAuth configuration missing", zap.String("component", "main"))
		return
	}

	googleOAuthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			// Calendar APIs
			"https://www.googleapis.com/auth/calendar",
			"https://www.googleapis.com/auth/calendar.events",

			// Gmail APIs - Full access for comprehensive email management
			// Full Gmail access (read, send, delete, modify)
			"https://www.googleapis.com/auth/gmail.modify",
			"https://www.googleapis.com/auth/gmail.compose",  // Create drafts
			"https://www.googleapis.com/auth/gmail.insert",   // Insert messages
			"https://www.googleapis.com/auth/gmail.labels",   // Manage labels
			"https://www.googleapis.com/auth/gmail.metadata", // Metadata access

			// Google Drive API - for attachments
			// Access to files created/modified by the app
			"https://www.googleapis.com/auth/drive.file",

			// Google People API - for contacts
			"https://www.googleapis.com/auth/contacts.readonly", // Read contacts

			// User info
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}

	// 4. Initialiseer de 'Store' Laag (AANGEPAST)
	// De store heeft nu de oauth config nodig om zelf tokens te verversen.
	dbStore := store.NewStore(pool, googleOAuthConfig, log)

	// 5. Initialiseer de Worker (geef de store mee)
	appWorker, err := worker.NewWorker(dbStore, log)
	if err != nil {
		log.Error("could not initialize worker", zap.Error(err))
		return
	}

	// 6. Start de Worker in de achtergrond
	appWorker.Start()

	// 7. Initialiseer de API Server (geef de store, logger en config mee)
	apiServer := api.NewServer(dbStore, log, googleOAuthConfig)

	// 8. Start de HTTP Server (op de voorgrond)
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080" // Default poort
	}

	log.Info("starting API server", zap.String("port", port), zap.String("component", "main"))

	// GOED: Maak een server met timeouts
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      apiServer.Router,
		ReadTimeout:  5 * time.Second,   // 5 sec om headers te lezen
		WriteTimeout: 10 * time.Second,  // 10 sec om response te schrijven
		IdleTimeout:  120 * time.Second, // 120 sec max keep-alive
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("could not start server", zap.Error(err))
	}
}
