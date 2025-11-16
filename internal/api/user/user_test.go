package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap" // <-- TOEGEVOEGD
)

// Use the exported UserContextKey from common

func TestHandleGetMe(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create a test user
	userID := uuid.New()
	testUser := domain.User{
		BaseEntity: domain.BaseEntity{
			ID:        userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Email: "test@example.com",
		Name:  stringPtr("Test User"),
	}

	// Set up the mock to return the test user
	mockStore.On("GetUserByID", mock.Anything, userID).Return(testUser, nil)

	// Create a request with context containing user ID
	req, err := http.NewRequest("GET", "/api/v1/me", http.NoBody)
	assert.NoError(t, err)

	// Add user ID to context (using the same key as in common.go)
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGetMe(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response domain.User
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, testUser.ID, response.ID)
	assert.Equal(t, testUser.Email, response.Email)
	assert.Equal(t, testUser.Name, response.Name)

	// Check the content type
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

func TestHandleGetMe_NoUserIDInContext(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create a request without user ID in context
	req, err := http.NewRequest("GET", "/api/v1/me", http.NoBody)
	assert.NoError(t, err)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGetMe(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// Check the response body contains error
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

func TestHandleGetMe_StoreError(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()

	// Set up the mock to return an error
	mockStore.On("GetUserByID", mock.Anything, userID).Return(domain.User{}, assert.AnError)

	// Create a request with context containing user ID
	req, err := http.NewRequest("GET", "/api/v1/me", http.NoBody)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGetMe(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// Check the response body contains error
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
