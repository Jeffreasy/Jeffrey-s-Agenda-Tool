package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestConnectDB_NoDatabaseURL(t *testing.T) {
	// Create test logger
	observedCore, _ := observer.New(zapcore.DebugLevel)
	testLogger := zap.New(observedCore)

	// Ensure DATABASE_URL is not set
	t.Setenv("DATABASE_URL", "")

	pool, err := ConnectDB(testLogger)
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "DATABASE_URL environment variable is not set")
}

func TestConnectDB_InvalidDatabaseURL(t *testing.T) {
	// Create test logger
	observedCore, _ := observer.New(zapcore.DebugLevel)
	testLogger := zap.New(observedCore)

	// Set an invalid DATABASE_URL
	t.Setenv("DATABASE_URL", "invalid-url")

	pool, err := ConnectDB(testLogger)
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "unable to create connection pool")
}

// Note: Testing successful connection would require a real database
// For integration tests, you would set up a test database and use a valid DATABASE_URL

