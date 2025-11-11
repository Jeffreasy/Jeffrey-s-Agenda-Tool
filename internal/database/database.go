package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectDB maakt en test een verbinding met de database.
// We gebruiken een *pgxpool.Pool voor connection pooling.
func ConnectDB() (*pgxpool.Pool, error) {
	// We gaan ervan uit dat de config al is geladen (zie main.go)
	dbUrl := os.Getenv("DATABASE_URL")
	log.Println("Connecting to DB URL:", dbUrl)
	if dbUrl == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	// Ping de database om de verbinding te verifiëren
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	log.Println("Successfully connected to database.")
	return pool, nil
}
