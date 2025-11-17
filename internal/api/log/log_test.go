package log

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

func TestHandleGetAutomationLogs(t *testing.T) {
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

	ruleID := uuid.New()
	logs := []domain.AutomationLog{
		{
			ID:                 1,
			ConnectedAccountID: accountID,
			RuleID:             &ruleID,
			Timestamp:          time.Now(),
			Status:             domain.LogSuccess,
			TriggerDetails:     json.RawMessage(`{"event_id": "test"}`),
			ActionDetails:      json.RawMessage(`{"created_event": "test"}`),
			ErrorMessage:       "",
		},
	}

	// Set up the mocks
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("GetLogsForAccount", mock.Anything, accountID, 50).Return(logs, nil)

	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/logs", http.NoBody)
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
	handler := HandleGetAutomationLogs(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response []domain.AutomationLog
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, int64(1), response[0].ID)
	assert.Equal(t, domain.LogSuccess, response[0].Status)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}

func TestHandleGetAutomationLogs_InvalidAccountID(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()

	req, err := http.NewRequest("GET", "/api/v1/accounts/invalid-id/logs", http.NoBody)
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
	handler := HandleGetAutomationLogs(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGetAutomationLogs_AccountNotFound(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()

	// Set up the mock to return an error (account not found)
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(domain.ConnectedAccount{}, assert.AnError)

	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/logs", http.NoBody)
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
	handler := HandleGetAutomationLogs(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

func TestHandleGetAutomationLogs_Unauthorized(t *testing.T) {
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

	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/logs", http.NoBody)
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
	handler := HandleGetAutomationLogs(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Verify that the mock was called
	mockStore.AssertExpectations(t)
}

// Helper function to create string pointer
// VERWIJDERD: Deze functie werd niet gebruikt in dit bestand.
// func stringPtr(s string) *string {
//     return &s
// }
