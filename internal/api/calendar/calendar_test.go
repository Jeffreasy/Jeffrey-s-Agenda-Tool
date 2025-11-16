package calendar

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
    // "agenda-automator-api/internal/logger" // <-- VERWIJDERD (niet meer nodig)

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "go.uber.org/zap/zaptest/observer"
    "golang.org/x/oauth2"
    "google.golang.org/api/calendar/v3"
)

func TestHandleCreateEvent(t *testing.T) {
    // Create test logger
    observedCore, _ := observer.New(zapcore.DebugLevel)
    testLogger := zap.New(observedCore)

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create test data
    accountID := uuid.New()
    userID := uuid.New()

    // Mock the GetValidTokenForAccount call
    mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

    // Create a test event
    testEvent := &calendar.Event{
        Summary:     "Test Event",
        Description: "Test Description",
        Start: &calendar.EventDateTime{
            DateTime: time.Now().Format(time.RFC3339),
        },
        End: &calendar.EventDateTime{
            DateTime: time.Now().Add(time.Hour).Format(time.RFC3339),
        },
    }

    // Create request body
    body, _ := json.Marshal(testEvent)

    // Create request
    req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/calendar/events", bytes.NewReader(body))
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
    // AANGEPAST: testLogger wordt nu meegegeven
    handler := HandleCreateEvent(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Since we can't easily mock the Google Calendar client,
    // we test that the handler processes the request without panicking
    // and that the mock store was called
    mockStore.AssertExpectations(t)
}

func TestHandleCreateEvent_InvalidAccountID(t *testing.T) {
    // Create test logger
    observedCore, _ := observer.New(zapcore.DebugLevel)
    testLogger := zap.New(observedCore)

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create test data
    userID := uuid.New()

    // Create request
    req, err := http.NewRequest("POST", "/api/v1/accounts/invalid-id/calendar/events", nil)
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
    handler := HandleCreateEvent(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Check response
    assert.Equal(t, http.StatusBadRequest, rr.Code)

    var response map[string]string
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Contains(t, response, "error")
    assert.Contains(t, response["error"], "Ongeldig account ID")
}

func TestHandleCreateEvent_InvalidJSON(t *testing.T) {
    // Create test logger
    observedCore, _ := observer.New(zapcore.DebugLevel)
    testLogger := zap.New(observedCore)

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create test data
    accountID := uuid.New()
    userID := uuid.New()

    // Create invalid JSON
    body := []byte(`{"invalid": json}`)

    // Create request
    req, err := http.NewRequest("POST", "/api/v1/accounts/"+accountID.String()+"/calendar/events", bytes.NewReader(body))
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
    handler := HandleCreateEvent(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Check response
    assert.Equal(t, http.StatusBadRequest, rr.Code)

    var response map[string]string
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Contains(t, response, "error")
    assert.Contains(t, response["error"], "Ongeldige request body")
}

func TestHandleGetCalendarEvents(t *testing.T) {
    // Create test logger
    observedCore, _ := observer.New(zapcore.DebugLevel)
    testLogger := zap.New(observedCore)

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create test data
    accountID := uuid.New()
    userID := uuid.New()

    // Mock the GetValidTokenForAccount call
    mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

    // Create request
    req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/calendar/events", nil)
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
    handler := HandleGetCalendarEvents(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Test that the handler processes the request
    // (We can't easily mock the Google Calendar client, so we test the request processing)
    mockStore.AssertExpectations(t)
}

func TestHandleGetCalendarEvents_InvalidAccountID(t *testing.T) {
    // AANGEPAST: Maak een Nop-logger (doet niets)
    testLogger := zap.NewNop()

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create test data
    userID := uuid.New()

    // Create request
    req, err := http.NewRequest("GET", "/api/v1/accounts/invalid-id/calendar/events", nil)
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
    // AANGEPAST: testLogger meegegeven
    handler := HandleGetCalendarEvents(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Check response
    assert.Equal(t, http.StatusBadRequest, rr.Code)

    var response map[string]string
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Contains(t, response, "error")
    assert.Contains(t, response["error"], "Ongeldig account ID")
}

func TestHandleListCalendars(t *testing.T) {
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
    req, err := http.NewRequest("GET", "/api/v1/accounts/"+accountID.String()+"/calendars", nil)
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
    // AANGEPAST: testLogger meegegeven
    handler := HandleListCalendars(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Test that the handler processes the request
    mockStore.AssertExpectations(t)
}

func TestHandleGetAggregatedEvents(t *testing.T) {
    // AANGEPAST: Maak een Nop-logger
    testLogger := zap.NewNop()

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create test data
    userID := uuid.New()
    accountID := uuid.New()

    // Mock the store calls
    mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(domain.ConnectedAccount{
        ID:     accountID,
        UserID: userID,
    }, nil)
    mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

    // Create request body
    reqBody := map[string]interface{}{
        "accounts": []map[string]string{
            {
                "accountId":  accountID.String(),
                "calendarId": "primary",
            },
        },
    }
    body, _ := json.Marshal(reqBody)

    // Create request
    req, err := http.NewRequest("POST", "/api/v1/calendar/aggregated-events", bytes.NewReader(body))
    assert.NoError(t, err)

    // Add user ID to context
    ctx := context.WithValue(req.Context(), common.UserContextKey, userID)
    req = req.WithContext(ctx)

    // Create response recorder
    rr := httptest.NewRecorder()

    // Call handler
    // AANGEPAST: testLogger meegegeven
    handler := HandleGetAggregatedEvents(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Test that the handler processes the request
    mockStore.AssertExpectations(t)
}

func TestHandleGetAggregatedEvents_NoAuth(t *testing.T) {
    // AANGEPAST: Maak een Nop-logger
    testLogger := zap.NewNop()

    // Create a mock store
    mockStore := &store.MockStore{}

    // Create request
    req, err := http.NewRequest("POST", "/api/v1/calendar/aggregated-events", nil)
    assert.NoError(t, err)

    // Create response recorder
    rr := httptest.NewRecorder()

    // Call handler
    // AANGEPAST: testLogger meegegeven
    handler := HandleGetAggregatedEvents(mockStore, testLogger)
    handler.ServeHTTP(rr, req)

    // Check response
    assert.Equal(t, http.StatusUnauthorized, rr.Code)

    var response map[string]string
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Contains(t, response, "error")
}

