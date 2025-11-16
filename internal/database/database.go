package database

import (
	"agenda-automator-api/internal/logger"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Querier defines the database operations used by stores and migrations
// Both *pgxpool.Pool and pgx.Tx implement this interface
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ConnectDB maakt en test een verbinding met de database.
func ConnectDB(log *zap.Logger) (*pgxpool.Pool, error) {
	start := time.Now()
	dbUrl := os.Getenv("DATABASE_URL")
	log.Info("connecting to database", zap.String("component", "database"))
	if dbUrl == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		logger.LogDuration(log, "database_connection_pool_creation", time.Since(start).Milliseconds(), zap.Error(err), zap.String("component", "database"))
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	pingStart := time.Now()
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		logger.LogDuration(log, "database_ping", time.Since(pingStart).Milliseconds(), zap.Error(err), zap.String("component", "database"))
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	totalDuration := time.Since(start)
	logger.LogDuration(log, "database_connection_total", totalDuration.Milliseconds(), zap.String("component", "database"))
	log.Info("successfully connected to database", zap.String("component", "database"))
	return pool, nil
}
