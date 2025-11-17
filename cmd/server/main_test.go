package main

import (
	// <-- Zorg dat deze import aanwezig is
	"testing"
	"time" // <-- HIER TOEGEVOEGD

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestRun_Success test het 'happy path' van de applicatie-setup.
func TestRun_Success(t *testing.T) {
	// --- Arrange ---

	// 1. Stel de vereiste environment variables in
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")
	t.Setenv("OAUTH_REDIRECT_URL", "http://test/callback")
	t.Setenv("API_PORT", "8181") // Gebruik een andere poort voor de test

	// 2. Maak de mock dependencies
	logger := zap.NewNop() // Een logger die niets doet

	mockPool, err := pgxmock.NewPool() // Een mock database pool
	require.NoError(t, err)
	defer mockPool.Close()

	// De worker start en roept doWork() -> checkAccounts() -> GetActiveAccounts() aan.
	// We moeten die call mocken, anders faalt de test.
	rows := pgxmock.NewRows([]string{"id", "user_id", "email"}) // Lege rijen

	// We gebruiken een string die als regex wordt geïnterpreteerd.
	// "SELECT .*" matcht elke SELECT query (inclusief GetActiveAccounts).
	mockPool.ExpectQuery("SELECT .*").
		WillReturnRows(rows)

	// --- Act ---
	// Roep de 'run' functie aan (die nu testbaar is)
	server, err := run(logger, mockPool)

	// --- HIER IS DE FIX ---
	// Wacht heel even zodat de worker goroutine kan opstarten
	// en de GetActiveAccounts query kan uitvoeren.
	time.Sleep(50 * time.Millisecond)
	// --- EINDE FIX ---

	// --- Assert ---
	assert.NoError(t, err)   // Er mag geen error zijn
	assert.NotNil(t, server) // We moeten een server terugkrijgen

	// Controleer of de server correct is geconfigureerd
	assert.Equal(t, ":8181", server.Addr)
	assert.NotNil(t, server.Handler) // De router moet ingesteld zijn

	// Controleer of de mock call daadwerkelijk is gebeurd
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// TestRun_MissingOAuthEnv test of de setup faalt als env vars missen
func TestRun_MissingOAuthEnv(t *testing.T) {
	// --- Arrange ---
	// We zetten *geen* env vars

	// Maak de mock dependencies
	logger := zap.NewNop()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	// --- Act ---
	server, err := run(logger, mockPool)

	// --- Assert ---
	assert.Error(t, err)  // We verwachten nu een error
	assert.Nil(t, server) // We mogen geen server terugkrijgen

	assert.Contains(t, err.Error(), "google OAuth configuration missing")
}
