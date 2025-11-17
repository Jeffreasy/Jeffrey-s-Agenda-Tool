package account

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/database" // <-- IMPORT TOEGEVOEGD
	"agenda-automator-api/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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
	"golang.org/x/oauth2"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// --- CONFIGURATIE ---
// TODO: Pas dit pad aan naar de locatie van je migratiebestanden
const (
	migrationsPath = "file://../../../db/migrations"
	// Dit is een 32-byte test-key. Gebruik deze NOOIT in productie.
	testEncryptionKey = "01234567890123456789012345678901"
)

// --- GLOBALE TEST VARIABELEN ---
var (
	testPool        *pgxpool.Pool
	testStore       AccountStorer
	mockOAuthServer *httptest.Server
	testUser        domain.User // Een gebruiker die we als 'seed' gebruiken
)

// TestMain wordt één keer uitgevoerd voor alle tests in deze package.
// Het start de database, draait migraties, en zet de test-store op.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Setup Crypto Key (cruciaal voor encryptie/decryptie)
	if err := os.Setenv("ENCRYPTION_KEY", testEncryptionKey); err != nil {
		log.Fatalf("Could not set ENCRYPTION_KEY: %v", err)
	}

	// 2. Setup Mock OAuth Server
	mockOAuthServer = setupMockOAuthServer()
	defer mockOAuthServer.Close()

	// 3. Setup Test Database
	pgContainer, dbURL, err := setupTestDatabase(ctx)
	if err != nil {
		log.Fatalf("Could not set up test database: %v", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("Could not terminate test database: %v", err)
		}
	}()

	// 4. Maak verbinding met de test database
	testPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Could not connect to test database: %v", err)
	}
	defer testPool.Close()

	// 5. Maak de test-store
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://google.com/auth",
			TokenURL:  mockOAuthServer.URL, // <-- Belangrijk
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: "http://localhost/callback",
		Scopes:      []string{"test-scope"},
	}

	// We geven testPool (een *pgxpool.Pool) mee.
	// Dit werkt omdat *pgxpool.Pool de database.Querier interface implementeert.
	testStore = NewAccountStore(testPool, oauthConfig, zap.NewNop())

	// 6. Seed een testgebruiker
	testUser, err = seedTestUser(ctx, testPool)
	if err != nil {
		log.Fatalf("Could not seed test user: %v", err)
	}

	// 7. Run de tests
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
		return nil, "", fmt.Errorf("failed to create migrate instance: %w (path: %s)", err, migrationsPath)
	}
	if err := m.Up(); err != nil {
		return nil, "", fmt.Errorf("failed to run migrations: %w", err)
	}

	return pgContainer, connStr, nil
}

// setupMockOAuthServer maakt een fake Google token endpoint
func setupMockOAuthServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		grantType := r.Form.Get("grant_type")
		refreshToken := r.Form.Get("refresh_token")

		if grantType != "refresh_token" {
			http.Error(w, "unsupported_grant_type", http.StatusBadRequest)
			return
		}

		if refreshToken == "revoke_me" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "invalid_grant",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new_access_token_from_mock",
			"refresh_token": "new_refresh_token_from_mock",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
}

// seedTestUser maakt een 'users' record aan
func seedTestUser(ctx context.Context, pool *pgxpool.Pool) (domain.User, error) {
	var user domain.User
	user.ID = uuid.New()
	user.Email = fmt.Sprintf("test.user.%s@example.com", user.ID.String()[:8])
	name := "Test User"
	user.Name = &name
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	query := `
	INSERT INTO users (id, email, name, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id, email, name, created_at, updated_at
	`
	err := pool.QueryRow(ctx, query, user.ID, user.Email, user.Name, user.CreatedAt, user.UpdatedAt).Scan(
		&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf("failed to seed user: %w", err)
	}
	return user, nil
}

// --- DE TESTS ---

// TestNewAccountStore
func TestNewAccountStore(t *testing.T) {
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	// Geef een 'nil' interface mee
	var nilQuerier database.Querier
	store := NewAccountStore(nilQuerier, oauthConfig, nil)
	assert.NotNil(t, store)

	accountStore, ok := store.(*AccountStore)
	assert.True(t, ok)

	// FIX: De refactor van 'pool' naar 'db' is hier doorgevoerd
	assert.Nil(t, accountStore.db)
	assert.Equal(t, oauthConfig, accountStore.googleOAuthConfig)
}

// TestErrTokenRevoked
func TestErrTokenRevoked(t *testing.T) {
	assert.NotNil(t, ErrTokenRevoked)
	assert.Contains(t, ErrTokenRevoked.Error(), "token access has been revoked")
}

// TestAccountStore_Integration_FullFlow
func TestAccountStore_Integration_FullFlow(t *testing.T) {
	ctx := context.Background()
	require := require.New(t)
	assert := assert.New(t)

	otherUser, err := seedTestUser(ctx, testPool)
	require.NoError(err)

	var accountID uuid.UUID
	var acc domain.ConnectedAccount

	t.Run("1. Upsert (Create)", func(t *testing.T) {
		params := UpsertConnectedAccountParams{
			UserID:         testUser.ID,
			Provider:       domain.ProviderGoogle,
			Email:          "test.account@gmail.com",
			ProviderUserID: "provider123",
			AccessToken:    "access_token_1",
			RefreshToken:   "refresh_token_1",
			TokenExpiry:    time.Now().Add(time.Hour),
			Scopes:         []string{"scope1", "scope2"},
		}
		acc, err = testStore.UpsertConnectedAccount(ctx, params)
		require.NoError(err)
		accountID = acc.ID

		assert.Equal(params.UserID, acc.UserID)
		assert.Equal(params.Email, acc.Email)
		assert.Equal(domain.StatusActive, acc.Status)
		assert.Equal(params.Scopes, acc.Scopes)
		assert.NotNil(acc.AccessToken)
		assert.NotNil(acc.RefreshToken)
	})

	t.Run("2. Get By ID and Verify Encryption", func(t *testing.T) {
		fetchedAcc, err := testStore.GetConnectedAccountByID(ctx, accountID)
		require.NoError(err)
		assert.Equal(acc.Email, fetchedAcc.Email)
		assert.Equal(acc.ID, fetchedAcc.ID)

		var encryptedToken []byte
		err = testPool.QueryRow(ctx, "SELECT access_token FROM connected_accounts WHERE id = $1", accountID).Scan(&encryptedToken)
		require.NoError(err)
		assert.NotEqual("access_token_1", string(encryptedToken), "Token in DB should be encrypted!")

		decrypted, err := crypto.Decrypt(fetchedAcc.AccessToken)
		require.NoError(err)
		assert.Equal("access_token_1", string(decrypted))
	})

	t.Run("3. Upsert (Update)", func(t *testing.T) {
		params := UpsertConnectedAccountParams{
			UserID:         testUser.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: "provider123",
			AccessToken:    "access_token_2",
			RefreshToken:   "refresh_token_2",
			TokenExpiry:    time.Now().Add(2 * time.Hour),
			Scopes:         []string{"scope3"},
		}
		updatedAcc, err := testStore.UpsertConnectedAccount(ctx, params)
		require.NoError(err)

		assert.Equal(accountID, updatedAcc.ID)
		assert.Equal(params.Scopes, updatedAcc.Scopes)

		decrypted, err := crypto.Decrypt(updatedAcc.AccessToken)
		require.NoError(err)
		assert.Equal("access_token_2", string(decrypted))
	})

	t.Run("4. Verify Ownership", func(t *testing.T) {
		err := testStore.VerifyAccountOwnership(ctx, accountID, testUser.ID)
		assert.NoError(err)

		err = testStore.VerifyAccountOwnership(ctx, accountID, otherUser.ID)
		assert.Error(err)
		assert.Contains(err.Error(), "forbidden")

		err = testStore.VerifyAccountOwnership(ctx, uuid.New(), testUser.ID)
		assert.Error(err)
	})

	t.Run("5. Get Accounts For User", func(t *testing.T) {
		_, err := testStore.UpsertConnectedAccount(ctx, UpsertConnectedAccountParams{
			UserID:         testUser.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: "provider456",
			AccessToken:    "access_token_3",
			TokenExpiry:    time.Now().Add(time.Hour),
		})
		require.NoError(err)

		accounts, err := testStore.GetAccountsForUser(ctx, testUser.ID)
		require.NoError(err)
		assert.Len(accounts, 2)

		accounts, err = testStore.GetAccountsForUser(ctx, otherUser.ID)
		require.NoError(err)
		assert.Len(accounts, 0)
	})

	t.Run("6. Update Status and Get Active", func(t *testing.T) {
		err := testStore.UpdateAccountStatus(ctx, accountID, domain.StatusRevoked)
		require.NoError(err)

		acc, err := testStore.GetConnectedAccountByID(ctx, accountID)
		require.NoError(err)
		assert.Equal(domain.StatusRevoked, acc.Status)

		activeAccounts, err := testStore.GetActiveAccounts(ctx)
		require.NoError(err)
		assert.Len(activeAccounts, 1)
		assert.NotEqual(accountID, activeAccounts[0].ID)
	})

	t.Run("7. Update Last Checked", func(t *testing.T) {
		err := testStore.UpdateAccountLastChecked(ctx, accountID)
		require.NoError(err)

		var lastChecked time.Time
		err = testPool.QueryRow(ctx, "SELECT last_checked FROM connected_accounts WHERE id = $1", accountID).Scan(&lastChecked)
		require.NoError(err)
		assert.WithinDuration(time.Now(), lastChecked, 5*time.Second)
	})

	t.Run("8. Delete Account", func(t *testing.T) {
		err := testStore.DeleteConnectedAccount(ctx, accountID)
		require.NoError(err)

		_, err = testStore.GetConnectedAccountByID(ctx, accountID)
		assert.Error(err)
		assert.Contains(err.Error(), "account not found")

		err = testStore.DeleteConnectedAccount(ctx, uuid.New())
		assert.Error(err)
		assert.Contains(err.Error(), "no account found")
	})
}

// TestAccountStore_Integration_TokenRefresh
func TestAccountStore_Integration_TokenRefresh(t *testing.T) {
	ctx := context.Background()
	require := require.New(t)
	assert := assert.New(t)

	t.Run("Valid Token", func(t *testing.T) {
		params := UpsertConnectedAccountParams{
			UserID:         testUser.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: "token_user_1",
			AccessToken:    "valid_access_token",
			RefreshToken:   "valid_refresh_token",
			TokenExpiry:    time.Now().Add(time.Hour),
		}
		acc, err := testStore.UpsertConnectedAccount(ctx, params)
		require.NoError(err)

		token, err := testStore.GetValidTokenForAccount(ctx, acc.ID)
		require.NoError(err)

		assert.Equal("valid_access_token", token.AccessToken)
		assert.Equal("valid_refresh_token", token.RefreshToken)
	})

	t.Run("Expired Token Refresh", func(t *testing.T) {
		params := UpsertConnectedAccountParams{
			UserID:         testUser.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: "token_user_2",
			AccessToken:    "expired_access_token",
			RefreshToken:   "good_refresh_token",
			TokenExpiry:    time.Now().Add(-time.Hour),
		}
		acc, err := testStore.UpsertConnectedAccount(ctx, params)
		require.NoError(err)

		token, err := testStore.GetValidTokenForAccount(ctx, acc.ID)
		require.NoError(err)

		assert.Equal("new_access_token_from_mock", token.AccessToken)
		assert.Equal("new_refresh_token_from_mock", token.RefreshToken)
		assert.WithinDuration(time.Now().Add(time.Hour), token.Expiry, 5*time.Second)

		dbAcc, err := testStore.GetConnectedAccountByID(ctx, acc.ID)
		require.NoError(err)

		decryptedAccess, err := crypto.Decrypt(dbAcc.AccessToken)
		require.NoError(err)
		assert.Equal("new_access_token_from_mock", string(decryptedAccess))

		decryptedRefresh, err := crypto.Decrypt(dbAcc.RefreshToken)
		require.NoError(err)
		assert.Equal("new_refresh_token_from_mock", string(decryptedRefresh))
	})

	t.Run("Revoked Token", func(t *testing.T) {
		params := UpsertConnectedAccountParams{
			UserID:         testUser.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: "token_user_3",
			AccessToken:    "expired_access_token",
			RefreshToken:   "revoke_me",
			TokenExpiry:    time.Now().Add(-time.Hour),
		}
		acc, err := testStore.UpsertConnectedAccount(ctx, params)
		require.NoError(err)

		token, err := testStore.GetValidTokenForAccount(ctx, acc.ID)

		assert.Error(err)
		assert.Nil(token)
		assert.Equal(ErrTokenRevoked, err)

		dbAcc, err := testStore.GetConnectedAccountByID(ctx, acc.ID)
		require.NoError(err)
		assert.Equal(domain.StatusRevoked, dbAcc.Status)
	})
}

// Test_getDecryptedToken
func Test_getDecryptedToken(t *testing.T) {
	encryptedAccess, err := crypto.Encrypt([]byte("access"))
	require.NoError(t, err)
	encryptedRefresh, err := crypto.Encrypt([]byte("refresh"))
	require.NoError(t, err)

	acc := domain.ConnectedAccount{
		AccessToken:  encryptedAccess,
		RefreshToken: encryptedRefresh,
		TokenExpiry:  time.Now(),
	}

	store := AccountStore{}

	token, err := store.getDecryptedToken(acc)
	require.NoError(t, err)

	assert.Equal(t, "access", token.AccessToken)
	assert.Equal(t, "refresh", token.RefreshToken)
	assert.Equal(t, "Bearer", token.TokenType)
}
