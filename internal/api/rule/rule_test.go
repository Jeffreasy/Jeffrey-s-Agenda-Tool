package rule

import (
	"bytes"
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

func TestHandleCreateRule(t *testing.T) {
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

	ruleReq := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			Name:              "Test Rule",
			TriggerConditions: json.RawMessage(`{"summary_equals": "test"}`),
			ActionParams:      json.RawMessage(`{"offset_minutes": 15}`),
		},
	}

	expectedRule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				BaseEntity: domain.BaseEntity{
					ID:        uuid.New(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				ConnectedAccountID: accountID,
			},
			Name:              ruleReq.Name,
			TriggerConditions: ruleReq.TriggerConditions,
			ActionParams:      ruleReq.ActionParams,
		},
	}

	// Set up the mocks
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("CreateAutomationRule", mock.Anything, mock.MatchedBy(func(params store.CreateAutomationRuleParams) bool {
		return params.ConnectedAccountID == accountID && params.Name == ruleReq.Name
	})).Return(expectedRule, nil)

	// Create request body
	reqBody, _ := json.Marshal(ruleReq)
	req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/rules", bytes.NewBuffer(reqBody))
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
	handler := HandleCreateRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Check the response body
	var response domain.AutomationRule
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule.Name, response.Name)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}

func TestHandleCreateRule_InvalidAccountID(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()

	req, err := http.NewRequest("POST", "/api/v1/accounts/invalid-id/rules", nil)
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
	handler := HandleCreateRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGetRules(t *testing.T) {
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

	rules := []domain.AutomationRule{
		{
			BaseAutomationRule: domain.BaseAutomationRule{
				AccountEntity: domain.AccountEntity{
					ConnectedAccountID: accountID,
				},
				Name: "Test Rule 1",
			},
		},
	}

	// Set up the mocks
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("GetRulesForAccount", mock.Anything, accountID).Return(rules, nil)

	req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/rules", nil)
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
	handler := HandleGetRules(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response []domain.AutomationRule
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, "Test Rule 1", response[0].Name)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}

func TestHandleUpdateRule(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()
	ruleID := uuid.New()

	rule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				ConnectedAccountID: accountID,
			},
		},
	}

	account := domain.ConnectedAccount{
		ID:     accountID,
		UserID: userID,
	}

	updatedRule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				ConnectedAccountID: accountID,
			},
			Name: "Updated Rule",
		},
	}

	ruleReq := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			Name: "Updated Rule",
		},
	}

	// Set up the mocks
	mockStore.On("GetRuleByID", mock.Anything, ruleID).Return(rule, nil)
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("UpdateRule", mock.Anything, mock.MatchedBy(func(params store.UpdateRuleParams) bool {
		return params.RuleID == ruleID && params.Name == ruleReq.Name
	})).Return(updatedRule, nil)

	// Create request body
	reqBody, _ := json.Marshal(ruleReq)
	req, err := http.NewRequest("PUT", "/api/v1/rules/"+ruleID.String(), bytes.NewBuffer(reqBody))
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ruleId", ruleID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleUpdateRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response domain.AutomationRule
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Rule", response.Name)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}

func TestHandleDeleteRule(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()
	ruleID := uuid.New()

	rule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				ConnectedAccountID: accountID,
			},
		},
	}

	account := domain.ConnectedAccount{
		ID:     accountID,
		UserID: userID,
	}

	// Set up the mocks
	mockStore.On("GetRuleByID", mock.Anything, ruleID).Return(rule, nil)
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("DeleteRule", mock.Anything, ruleID).Return(nil)

	req, err := http.NewRequest("DELETE", "/api/v1/rules/"+ruleID.String(), nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ruleId", ruleID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleDeleteRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}

func TestHandleToggleRule(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	userID := uuid.New()
	accountID := uuid.New()
	ruleID := uuid.New()

	rule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				ConnectedAccountID: accountID,
			},
		},
	}

	account := domain.ConnectedAccount{
		ID:     accountID,
		UserID: userID,
	}

	toggledRule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				ConnectedAccountID: accountID,
			},
			IsActive: true,
		},
	}

	// Set up the mocks
	mockStore.On("GetRuleByID", mock.Anything, ruleID).Return(rule, nil)
	mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(account, nil)
	mockStore.On("ToggleRuleStatus", mock.Anything, ruleID).Return(toggledRule, nil)

	req, err := http.NewRequest("PUT", "/api/v1/rules/"+ruleID.String()+"/toggle", nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
	req = req.WithContext(ctx)

	// Set up chi router context for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ruleId", ruleID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Create a ResponseRecorder
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleToggleRule(mockStore, testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response domain.AutomationRule
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.IsActive)

	// Verify that the mocks were called
	mockStore.AssertExpectations(t)
}
