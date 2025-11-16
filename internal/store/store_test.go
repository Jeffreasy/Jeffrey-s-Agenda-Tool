package store

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func TestNewStore(t *testing.T) {
	// Test constructor with nil parameters
	testLogger := zap.NewNop()
	store := NewStore(nil, nil, testLogger)
	assert.NotNil(t, store)

	dbStore, ok := store.(*DBStore)
	assert.True(t, ok)
	assert.NotNil(t, dbStore)

	// Test that all sub-stores are initialized
	assert.NotNil(t, dbStore.userStore)
	assert.NotNil(t, dbStore.accountStore)
	assert.NotNil(t, dbStore.ruleStore)
	assert.NotNil(t, dbStore.logStore)
	assert.NotNil(t, dbStore.gmailStore)
}

func TestNewStore_WithParameters(t *testing.T) {
	// Test constructor with actual parameters
	pool := &pgxpool.Pool{}
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	testLogger := zap.NewNop()

	store := NewStore(pool, oauthConfig, testLogger)
	assert.NotNil(t, store)

	dbStore, ok := store.(*DBStore)
	assert.True(t, ok)
	assert.NotNil(t, dbStore)

	// Verify the stores are created with the correct parameters
	assert.NotNil(t, dbStore.userStore)
	assert.NotNil(t, dbStore.accountStore)
	assert.NotNil(t, dbStore.ruleStore)
	assert.NotNil(t, dbStore.logStore)
	assert.NotNil(t, dbStore.gmailStore)
}

func TestDBStore_ImplementsStorerInterface(t *testing.T) {
	// Test that DBStore implements the Storer interface
	var store Storer = &DBStore{}
	assert.NotNil(t, store)

	// This test ensures compile-time interface compliance
	// If DBStore doesn't implement Storer, this will fail at compile time
}
