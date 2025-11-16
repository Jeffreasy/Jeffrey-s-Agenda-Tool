package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/domain"

	// "agenda-automator-api/internal/logger" // <-- VERWIJDERD
	"agenda-automator-api/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap" // <-- TOEGEVOEGD
	"golang.org/x/oauth2"
)

func TestHandleGetGmailMessages(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the GetValidTokenForAccount call
	// AANGEPAST: Verwacht nu ook de logger
	mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

	// Create request
	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/gmail/messages", nil)
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleGetGmailMessages(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

func TestHandleGetGmailMessages_InvalidAccountID(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	userID := uuid.New()

	// Create request
	req, err := http.NewRequest("GET", "/api/v1/accounts/invalid-id/gmail/messages", nil)
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", "invalid-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleGetGmailMessages(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "Ongeldig account ID")
}

func TestHandleSendGmailMessage(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the GetValidTokenForAccount call
	mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

	// Create request body
	reqBody := map[string]interface{}{
		"to":      []string{"recipient@example.com"},
		"subject": "Test Subject",
		"body":    "Test Body",
		"isHtml":  false,
	}
	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/gmail/send", bytes.NewReader(body))
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleSendGmailMessage(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

func TestHandleSendGmailMessage_InvalidJSON(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Create invalid JSON
	body := []byte(`{"invalid": json}`)

	// Create request
	req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/gmail/send", bytes.NewReader(body))
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleSendGmailMessage(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "Ongeldige request body")
}

func TestHandleGetGmailLabels(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the GetValidTokenForAccount call
	mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

	// Create request
	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/gmail/labels", nil)
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleGetGmailLabels(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

func TestHandleCreateGmailDraft(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the GetValidTokenForAccount call
	mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

	// Create request body
	reqBody := map[string]interface{}{
		"to":      []string{"recipient@example.com"},
		"subject": "Test Draft Subject",
		"body":    "Test Draft Body",
		"isHtml":  false,
	}
	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/gmail/drafts", bytes.NewReader(body))
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleCreateGmailDraft(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

func TestHandleGetGmailDrafts(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the GetValidTokenForAccount call
	mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

	// Create request
	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/gmail/drafts", nil)
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleGetGmailDrafts(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

func TestHandleCreateGmailRule(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the store calls
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(domain.ConnectedAccount{
		ID:     accountID,
		UserID: userID,
	}, nil)
	mockStore.On("CreateGmailAutomationRule", mock.Anything, mock.Anything).Return(domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				BaseEntity: domain.BaseEntity{
					ID: uuid.New(),
				},
				ConnectedAccountID: accountID,
			},
			Name:     "Test Rule",
			IsActive: true,
		},
	}, nil)

	// Create request body
	reqBody := domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			Name:     "Test Rule",
			IsActive: true,
		},
		TriggerType: "new_message",
		ActionType:  "add_label",
	}
	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/gmail/rules", bytes.NewReader(body))
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleCreateGmailRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

func TestHandleCreateGmailRule_NoAuth(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()

	// Create request body
	reqBody := domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			Name:     "Test Rule",
			IsActive: true,
		},
		TriggerType: "new_message",
		ActionType:  "add_label",
	}
	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/gmail/rules", bytes.NewReader(body))
	assert.NoError(t, err)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleCreateGmailRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check response - should be 401 because no user ID in context
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

func TestHandleGetGmailRules(t *testing.T) {
	// AANGEPAST: Maak een Nop-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Create test data
	accountID := uuid.New()
	userID := uuid.New()

	// Mock the store calls
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(domain.ConnectedAccount{
		ID:     accountID,
		UserID: userID,
	}, nil)
	mockStore.On("GetGmailRulesForAccount", mock.Anything, accountID).Return([]domain.GmailAutomationRule{}, nil)

	// Create request
	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/gmail/rules", nil)
	assert.NoError(t, err)

	// Add user ID to context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Add URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	// AANGEPAST: Geef testLogger mee
	handler := HandleGetGmailRules(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Test that the handler processes the request
	mockStore.AssertExpectations(t)
}

