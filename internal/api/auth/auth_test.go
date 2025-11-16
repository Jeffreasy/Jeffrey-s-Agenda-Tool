package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap" // <-- TOEGEVOEGD
	"golang.org/x/oauth2"
)

func TestHandleGoogleLogin(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock OAuth config
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/api/v1/auth/google/callback",
		Scopes:       []string{"email", "profile"},
		Endpoint:     oauth2.Endpoint{}, // Will be set by the actual config
	}

	// Create a request
	req, err := http.NewRequest("GET", "/api/v1/auth/google/login", nil)
	assert.NoError(t, err)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGoogleLogin(oauthConfig, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code - should be redirect
	assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)

	// Check that a state cookie was set
	cookies := rr.Result().Cookies()
	var stateCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == oauthStateCookieName {
			stateCookie = cookie
			break
		}
	}
	assert.NotNil(t, stateCookie, "State cookie should be set")
	assert.NotEmpty(t, stateCookie.Value, "State cookie should have a value")
	assert.True(t, stateCookie.HttpOnly, "State cookie should be HttpOnly")
	assert.Equal(t, "/", stateCookie.Path, "State cookie should have correct path")
	assert.Greater(t, stateCookie.MaxAge, 0, "State cookie should have MaxAge set")

	// Check the Location header - should contain the auth URL
	location := rr.Header().Get("Location")
	assert.NotEmpty(t, location, "Location header should be set")
	// Note: We can't easily test the exact URL without mocking the OAuth config's AuthCodeURL method
}

func TestGenerateJWT(t *testing.T) {
	// Set the JWT secret for testing
	t.Setenv("JWT_SECRET_KEY", "test-secret-key")

	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	token, err := generateJWT(userID)

	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// The token should be a valid JWT with 3 parts separated by dots
	parts := strings.Split(token, ".")
	assert.Equal(t, 3, len(parts), "JWT should have 3 parts")
}

func TestGenerateJWT_NoSecret(t *testing.T) {
	// Unset the JWT secret
	t.Setenv("JWT_SECRET_KEY", "")

	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	token, err := generateJWT(userID)

	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "JWT_SECRET_KEY is niet ingesteld")
}
