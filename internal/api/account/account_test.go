package account

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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap" // <-- TOEGEVOEGD
)

func TestHandleGetConnectedAccounts(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()

	// Create test accounts
	accounts := []domain.ConnectedAccount{
		{
			ID:             accountID,
			UserID:         userID,
			Provider:       domain.ProviderGoogle,
			Email:          "test@example.com",
			ProviderUserID: "google-user-123",
			Status:         domain.StatusActive,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Set up the mock to return the accounts
	mockStore.On("GetAccountsForUser", mock.Anything, userID).Return(accounts, nil)

	// Create a request with context containing user ID
	req, err := http.NewRequest("GET", "/api/v1/accounts", nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGetConnectedAccounts(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response []domain.ConnectedAccount
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, accountID, response[0].ID)
	assert.Equal(t, "test@example.com", response[0].Email)

	// Check the content type
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

func TestHandleGetConnectedAccounts_NoUserID(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create a request without user ID in context
	req, err := http.NewRequest("GET", "/api/v1/accounts", nil)
	assert.NoError(t, err)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGetConnectedAccounts(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandleGetConnectedAccounts_StoreError(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()

	// Set up the mock to return an error
	mockStore.On("GetAccountsForUser", mock.Anything, userID).Return([]domain.ConnectedAccount{}, assert.AnError)

	// Create a request with context containing user ID
	req, err := http.NewRequest("GET", "/api/v1/accounts", nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleGetConnectedAccounts(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

func TestHandleDeleteConnectedAccount(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()

	account := domain.ConnectedAccount{
		ID:     accountID,
		UserID: userID,
	}

	// Set up the mocks
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("DeleteConnectedAccount", mock.Anything, accountID).Return(nil)

	// Create a request with context containing user ID and URL param
	req, err := http.NewRequest("DELETE", "/api/v1/accounts/"+accountID.String(), nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleDeleteConnectedAccount(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}

func TestHandleDeleteConnectedAccount_InvalidAccountID(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()

	// Create a request with invalid account ID
	req, err := http.NewRequest("DELETE", "/api/v1/accounts/invalid-id", nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleDeleteConnectedAccount(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDeleteConnectedAccount_AccountNotFound(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()

	// Set up the mock to return an error (account not found)
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(domain.ConnectedAccount{}, assert.AnError)

	// Create a request with context containing user ID and URL param
	req, err := http.NewRequest("DELETE", "/api/v1/accounts/"+accountID.String(), nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleDeleteConnectedAccount(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

func TestHandleDeleteConnectedAccount_Unauthorized(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	otherUserID := uuid.New()
	accountID := uuid.New()

	// Account belongs to different user
	account := domain.ConnectedAccount{
		ID:     accountID,
		UserID: otherUserID, // Different user
	}

	// Set up the mock
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)

	// Create a request with context containing user ID and URL param
	req, err := http.NewRequest("DELETE", "/api/v1/accounts/"+accountID.String(), nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleDeleteConnectedAccount(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}
