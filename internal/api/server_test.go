package api

import (
	"agenda-automator-api/internal/api/common"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAuthMiddleware(t *testing.T) {
	// Setup test environment
	testLogger := zap.NewNop()

	// Set JWT secret for testing
	originalSecret := os.Getenv("JWT_SECRET_KEY")
	os.Setenv("JWT_SECRET_KEY", "test-secret-key")
	defer func() {
		if originalSecret != "" {
			os.Setenv("JWT_SECRET_KEY", originalSecret)
		} else {
			os.Unsetenv("JWT_SECRET_KEY")
		}
	}()

	// Create a minimal server just for the middleware
	server := &Server{Logger: testLogger}

	t.Run("No Authorization header", func(t *testing.T) {
		handlerCalled := false
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})
		middleware := server.authMiddleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), `"error":"Geen authenticatie header"`)
	})

	t.Run("Invalid JWT token", func(t *testing.T) {
		handlerCalled := false
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})
		middleware := server.authMiddleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), `"error":"Ongeldige token"`)
	})

	t.Run("Valid JWT token", func(t *testing.T) {
		// Create a valid JWT token
		userID := uuid.New()
		claims := jwt.MapClaims{
			"user_id": userID.String(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret-key"))

		var capturedUserID uuid.UUID
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user ID from the context that was set by middleware
			var err error
			capturedUserID, err = getUserIDFromContextForTest(r.Context())
			assert.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		})
		middleware := server.authMiddleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, userID, capturedUserID)
	})

	t.Run("JWT with invalid user_id claim", func(t *testing.T) {
		claims := jwt.MapClaims{
			"user_id": 12345, // Should be string, not number
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret-key"))

		handlerCalled := false
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})
		middleware := server.authMiddleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), `"error":"Geen user ID in token"`)
	})

	t.Run("JWT with invalid UUID format", func(t *testing.T) {
		claims := jwt.MapClaims{
			"user_id": "not-a-uuid",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret-key"))

		handlerCalled := false
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})
		middleware := server.authMiddleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), `"error":"Ongeldig user ID"`)
	})
}

// Helper function to extract user ID from context for testing
func getUserIDFromContextForTest(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(common.UserContextKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, http.ErrNoCookie
	}
	return userID, nil
}
