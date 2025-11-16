package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestNewAccountStore(t *testing.T) {
	// Create OAuth config
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	// Test constructor with nil pool (for basic constructor test)
	store := NewAccountStore(nil, oauthConfig, nil)
	assert.NotNil(t, store)
	assert.Nil(t, store.pool)
	assert.Equal(t, oauthConfig, store.googleOAuthConfig)
}

func TestErrTokenRevoked(t *testing.T) {
	// Test that the error variable is properly defined
	assert.NotNil(t, ErrTokenRevoked)
	assert.Contains(t, ErrTokenRevoked.Error(), "token access has been revoked")
}
