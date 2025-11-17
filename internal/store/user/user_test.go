package user

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// --- CONFIGURATIE ---
const (
	migrationsPath = "file://../../../db/migrations"
)

// --- GLOBALE TEST VARIABELEN ---
var (
	testUserPool  *pgxpool.Pool
	testUserStore UserStorer
)

// TestMain wordt één keer uitgevoerd voor alle tests in deze package.
// Het start de database, draait migraties, en zet de test-store op.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Setup Test Database
	pgContainer, dbURL, err := setupTestDatabase(ctx)
	if err != nil {
		log.Fatalf("Could not set up test database: %v", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("Could not terminate test database: %v", err)
		}
	}()

	// 2. Maak verbinding met de test database
	testUserPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Could not connect to test database: %v", err)
	}
	defer testUserPool.Close()

	// 3. Maak de test-store
	testUserStore = NewUserStore(testUserPool)

	// 4. Run de tests
	code := m.Run()
	os.Exit(code)
}

// setupTestDatabase start een Postgres container en past migraties toe.
func setupTestDatabase(ctx context.Context) (testcontainers.Container, string, error) {
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("test-user"),
		postgres.WithPassword("test-password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Minute),
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	// Draai migraties
	m, err := migrate.New(migrationsPath, connStr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create migrate instance: %w", err)
	}
	if err := m.Up(); err != nil {
		return nil, "", fmt.Errorf("failed to run migrations: %w", err)
	}

	return pgContainer, connStr, nil
}

// --- TESTS ---

func TestNewUserStore(t *testing.T) {
	// Test constructor with nil pool
	store := NewUserStore(nil)
	assert.NotNil(t, store)
	assert.Implements(t, (*UserStorer)(nil), store)

	// Test constructor with a pool
	pool := &pgxpool.Pool{}
	store = NewUserStore(pool)
	assert.NotNil(t, store)
	assert.Implements(t, (*UserStorer)(nil), store)
}

func TestUserStore_Integration_FullFlow(t *testing.T) {
	ctx := context.Background()
	require := require.New(t)
	assert := assert.New(t)

	var userID uuid.UUID
	var user domain.User

	t.Run("1. Create User", func(t *testing.T) {
		email := fmt.Sprintf("test.user.%s@example.com", uuid.New().String()[:8])
		name := "Test User"

		u, err := testUserStore.CreateUser(ctx, email, name)
		require.NoError(err)
		userID = u.ID
		user = u

		assert.Equal(email, u.Email)
		assert.Equal(&name, u.Name)
		assert.NotNil(u.ID)
		assert.NotZero(u.CreatedAt)
		assert.NotZero(u.UpdatedAt)
	})

	t.Run("2. Get User By ID", func(t *testing.T) {
		fetchedUser, err := testUserStore.GetUserByID(ctx, userID)
		require.NoError(err)

		assert.Equal(user.ID, fetchedUser.ID)
		assert.Equal(user.Email, fetchedUser.Email)
		assert.Equal(user.Name, fetchedUser.Name)
		assert.Equal(user.CreatedAt, fetchedUser.CreatedAt)
		assert.Equal(user.UpdatedAt, fetchedUser.UpdatedAt)
	})

	t.Run("3. Create User with Existing Email (Upsert)", func(t *testing.T) {
		// Try to create user with same email but different name
		newName := "Updated Test User"
		updatedUser, err := testUserStore.CreateUser(ctx, user.Email, newName)
		require.NoError(err)

		// Should return the same user with updated name
		assert.Equal(user.ID, updatedUser.ID)
		assert.Equal(user.Email, updatedUser.Email)
		assert.Equal(&newName, updatedUser.Name)
	})

	t.Run("4. Get Non-existent User", func(t *testing.T) {
		_, err := testUserStore.GetUserByID(ctx, uuid.New())
		assert.Error(err)
		assert.Contains(err.Error(), "user not found")
	})

	t.Run("5. Delete User", func(t *testing.T) {
		err := testUserStore.DeleteUser(ctx, userID)
		require.NoError(err)

		// Try to get the deleted user
		_, err = testUserStore.GetUserByID(ctx, userID)
		assert.Error(err)
		assert.Contains(err.Error(), "user not found")
	})

	t.Run("6. Delete Non-existent User", func(t *testing.T) {
		err := testUserStore.DeleteUser(ctx, uuid.New())
		assert.Error(err)
		assert.Contains(err.Error(), "no user found")
	})
}

func TestUserStore_Integration_MultipleUsers(t *testing.T) {
	ctx := context.Background()
	require := require.New(t)
	assert := assert.New(t)

	// Create multiple users
	var users []domain.User
	for i := 0; i < 3; i++ {
		email := fmt.Sprintf("multi.user.%d.%s@example.com", i, uuid.New().String()[:8])
		name := fmt.Sprintf("Multi User %d", i)

		user, err := testUserStore.CreateUser(ctx, email, name)
		require.NoError(err)
		users = append(users, user)
	}

	// Verify all users can be retrieved
	for _, user := range users {
		fetchedUser, err := testUserStore.GetUserByID(ctx, user.ID)
		require.NoError(err)
		assert.Equal(user.ID, fetchedUser.ID)
		assert.Equal(user.Email, fetchedUser.Email)
		assert.Equal(user.Name, fetchedUser.Name)
	}

	// Clean up
	for _, user := range users {
		err := testUserStore.DeleteUser(ctx, user.ID)
		require.NoError(err)
	}
}
