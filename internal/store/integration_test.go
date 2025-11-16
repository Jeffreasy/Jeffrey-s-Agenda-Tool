//go:build integration

package store

import (
	"agenda-automator-api/internal/database"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

func TestDatabaseIntegration(t *testing.T) {
	// De //go:build integration tag vervangt de testing.Short() check.

	ctx := context.Background()
	logger := zap.NewNop()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				// AANGEPAST: Verhogen naar 30 sec is veiliger
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)
	defer func() {
		// Gebruik context.Background() voor cleanup, niet de request-context
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	// Test 1: Run migrations
	t.Run("RunMigrations", func(t *testing.T) {
		// Set RUN_MIGRATIONS to true
		t.Setenv("RUN_MIGRATIONS", "true")

		err := database.RunMigrations(ctx, pool, logger)
		assert.NoError(t, err)
	})

	// Test 2: Verify tables were created
	t.Run("VerifyTablesCreated", func(t *testing.T) {
		tables := []string{
			"users",
			"connected_accounts",
			"automation_rules",
			"automation_logs",
			"gmail_automation_rules",
			"gmail_messages",
			"gmail_threads",
		}

		for _, table := range tables {
			var exists bool
			query := `SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = $1
			)`
			err := pool.QueryRow(ctx, query, table).Scan(&exists)
			assert.NoError(t, err, "Failed to check if table %s exists", table)
			assert.True(t, exists, "Table %s should exist", table)
		}
	})

	// Test 3: Test basic store operations
	t.Run("BasicStoreOperations", func(t *testing.T) {
		// Create store
		store := NewStore(pool, nil, logger)

		// Test user creation
		user, err := store.CreateUser(ctx, "test@example.com", "Test User")
		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Equal(t, "Test User", *user.Name) // Name is *string in the domain model

		// Test user retrieval
		retrievedUser, err := store.GetUserByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, retrievedUser.ID)
		assert.Equal(t, user.Email, retrievedUser.Email)

		// Test Gmail rule creation
		// Je moet een user en account aanmaken om foreign key constraints te voldoen
		accountID := uuid.New()
		_, err = pool.Exec(ctx, `INSERT INTO connected_accounts (
			id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status
		) VALUES ($1, $2, 'google', 'acc@test.com', '123', 'dummy_token',
			'dummy_refresh', now() + interval '1 hour', ARRAY['gmail'], 'active')`,
			accountID, user.ID)
		require.NoError(t, err)

		ruleParams := CreateGmailAutomationRuleParams{
			ConnectedAccountID: accountID, // Gebruik een bestaand account
			Name:               "Test Rule",
			Description:        stringPtr("Test description"),
			IsActive:           true,
			TriggerType:        "subject_match",
			TriggerConditions:  []byte(`{"keywords": ["test"]}`), // Moet []byte zijn, geen string
			ActionType:         "add_label",
			ActionParams:       []byte(`{"label": "Test"}`), // Moet []byte zijn, geen string
			Priority:           1,
		}

		rule, err := store.CreateGmailAutomationRule(ctx, ruleParams)
		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, rule.ID)
		assert.Equal(t, "Test Rule", rule.Name)
		assert.Equal(t, true, rule.IsActive)

		// Test rule retrieval
		rules, err := store.GetGmailRulesForAccount(ctx, ruleParams.ConnectedAccountID)
		assert.NoError(t, err)
		assert.Len(t, rules, 1)
		assert.Equal(t, rule.ID, rules[0].ID)
	})

	// Test 4: Test constraint violations
	t.Run("ConstraintViolations", func(t *testing.T) {
		// 'users' tabel heeft al 'test@example.com' van de vorige subtest
		// Probeer nogmaals met dezelfde email
		_, err := pool.Exec(ctx, "INSERT INTO users (email, name) VALUES ('test@example.com', 'Another User')")
		assert.Error(t, err, "Should fail due to unique constraint on email")
		assert.Contains(t, err.Error(), "duplicate key value violates unique constraint")
	})
}

func stringPtr(s string) *string {
	return &s
}
