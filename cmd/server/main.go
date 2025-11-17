package main

import (
	"context"
	"fmt"
	"time"

	"net/http"
	"os"

	// Interne packages
	"agenda-automator-api/internal/api"
	"agenda-automator-api/internal/database"
	"agenda-automator-api/internal/logger"
	"agenda-automator-api/internal/store"
	"agenda-automator-api/internal/worker"

	// Externe packages

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	// 1. Laad configuratie (.env)
	_ = godotenv.Load()

	// 1.5. Initialiseer logger
	log, err := logger.NewLogger()
	if err != nil {
		panic("Could not initialize logger: " + err.Error())
	}
	defer log.Sync()

	// 2. Maak verbinding met de Database
	pool, err := database.ConnectDB(log)
	if err != nil {
		log.Error("could not connect to the database", zap.Error(err))
		os.Exit(1) // Gebruik os.Exit voor een schone exit
	}
	defer pool.Close()

	// 2.5. Voer migraties uit
	if err = database.RunMigrations(context.Background(), pool, log); err != nil {
		log.Error("database migrations failed", zap.Error(err))
		os.Exit(1)
	}

	// 3. Roep de testbare run() functie aan
	// We geven de *concrete* pool mee, maar run() accepteert de interface
	server, err := run(log, pool)
	if err != nil {
		log.Error("application startup failed", zap.Error(err))
		os.Exit(1)
	}

	// 4. Start de HTTP Server (op de voorgrond)
	log.Info("starting API server", zap.String("addr", server.Addr), zap.String("component", "main"))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("could not start server", zap.Error(err))
		os.Exit(1)
	}
}

// run bevat alle testbare applicatie-opstartlogica.
// Het accepteert interfaces, wat het testbaar maakt.
func run(log *zap.Logger, pool database.Querier) (*http.Server, error) {
	// 3. Initialiseer de Gedeelde OAuth2 Config
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	redirectURL := os.Getenv("OAUTH_REDIRECT_URL")

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Error("Google OAuth configuration missing", zap.String("component", "run"))
		// We retourneren een error zodat de test kan falen
		// FIX: Geen hoofdletter (ST1005)
		return nil, fmt.Errorf("google OAuth configuration missing")
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

			// Gmail APIs
			"https://www.googleapis.com/auth/gmail.modify",
			"https://www.googleapis.com/auth/gmail.compose",
			"https://www.googleapis.com/auth/gmail.insert",
			"https://www.googleapis.com/auth/gmail.labels",
			"https://www.googleapis.com/auth/gmail.metadata",

			// Google Drive API
			"https://www.googleapis.com/auth/drive.file",

			// Google People API
			"https://www.googleapis.com/auth/contacts.readonly",

			// User info
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}

	// 4. Initialiseer de 'Store' Laag
	// FIX: De broken convertPoolToPgxPool is verwijderd.
	// We geven de 'pool' (database.Querier) interface direct door.
	dbStore := store.NewStore(pool, googleOAuthConfig, log)

	// 5. Initialiseer de Worker
	appWorker, err := worker.NewWorker(dbStore, log)
	if err != nil {
		log.Error("could not initialize worker", zap.Error(err))
		return nil, fmt.Errorf("could not initialize worker: %w", err)
	}

	// 6. Start de Worker in de achtergrond
	appWorker.Start()

	// 7. Initialiseer de API Server
	apiServer := api.NewServer(dbStore, log, googleOAuthConfig)

	// 8. Maak de HTTP Server
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080" // Default poort
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      apiServer.Router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 9. Return de server (main() zal ListenAndServe() aanroepen)
	return server, nil
}
