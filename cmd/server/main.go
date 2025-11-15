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
