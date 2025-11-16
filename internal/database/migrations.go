package database

import (
	"context"
	"os"
	"strings"

	"agenda-automator-api/db/migrations"

	"go.uber.org/zap"
)

// RunMigrations voert de database migraties uit op basis van de env var
func RunMigrations(ctx context.Context, db Querier, log *zap.Logger) error {
	run := os.Getenv("RUN_MIGRATIONS")
	if !strings.EqualFold(run, "true") {
		log.Info("skipping migrations (RUN_MIGRATIONS is not 'true')", zap.String("component", "migrations"))
		return nil
	}

	log.Info("running database migrations", zap.String("component", "migrations"))

	migrationSteps := []struct {
		name  string
		query string
	}{
		{"initial schema", migrations.InitialSchemaUp},
		{"Gmail schema", migrations.GmailSchemaUp},
		{"optimization indexes", migrations.OptimizationIndexesUp},
		{"table optimizations", migrations.TableOptimizationsUp},
		{"calendar optimizations", migrations.CalendarOptimizationsUp},
		{"connected accounts optimization", migrations.ConnectedAccountsOptimizationUp},
	}

	for _, step := range migrationSteps {
		if _, err := db.Exec(ctx, step.query); err != nil {
			log.Error(step.name+" migration failed", zap.Error(err))
			return err
		}
		log.Info(step.name+" migration applied successfully", zap.String("component", "migrations"))
	}

	log.Info("all database migrations applied successfully", zap.String("component", "migrations"))
	return nil
}
