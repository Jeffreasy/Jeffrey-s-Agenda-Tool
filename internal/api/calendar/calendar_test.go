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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

// --- Helpers ---

// setupTestHandlers centraliseert de setup voor mocks en de logger.
func setupTestHandlers(t *testing.T) (*store.MockStore, *zap.Logger) {
	t.Helper() // Markeer dit als een test helper
	mockStore := new(store.MockStore)
	testLogger := zap.NewNop()
	return mockStore, testLogger
}

// addTestContexts voegt de chi URL parameters en de user ID context toe.
func addTestContexts(req *http.Request, userID uuid.UUID, accountID string) *http.Request {
	// Voeg user ID toe aan context
	ctx := context.WithValue(req.Context(), common.UserContextKey, userID)

	// Voeg URL parameters toe
	rctx := chi.NewRouteContext()
	if accountID != "" {
		rctx.URLParams.Add("accountId", accountID)
	}
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	return req.WithContext(ctx)
}

// --- Tests ---

func TestHandleCreateEvent(t *testing.T) {
	accountID := uuid.New()
	userID := uuid.New()

	// De test-event body
	testEvent := &calendar.Event{
		Summary: "Test Event",
		Start:   &calendar.EventDateTime{DateTime: time.Now().Format(time.RFC3339)},
		End:     &calendar.EventDateTime{DateTime: time.Now().Add(time.Hour).Format(time.RFC3339)},
	}
	body, _ := json.Marshal(testEvent)

	// --- Test Scenario's ---
	t.Run("Happy Path - Kan niet mocken", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleCreateEvent(mockStore, testLogger)
		rr := httptest.NewRecorder()

		req, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
		req = addTestContexts(req, userID, accountID.String())

		// Mock de store call
		mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		// We kunnen de Google Client niet mocken (die wordt *in* de handler gemaakt).
		// We kunnen dus alleen checken of de handler niet crasht en de store call is gedaan.
		// De response code zal waarschijnlijk 500 zijn (omdat de Google call faalt),
		// tenzij je een http.Client injecteert (wat nu niet zo is).
		// Het belangrijkste is dat de mock call is geverifieerd.
		mockStore.AssertExpectations(t)
	})

	t.Run("Error - Invalid Account ID", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleCreateEvent(mockStore, testLogger)
		rr := httptest.NewRecorder()

		req, _ := http.NewRequest("POST", "/test", http.NoBody)
		req = addTestContexts(req, userID, "invalid-id") // Geen UUID

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Ongeldig account ID")
	})

	t.Run("Error - Invalid JSON Body", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleCreateEvent(mockStore, testLogger)
		rr := httptest.NewRecorder()

		req, _ := http.NewRequest("POST", "/test", bytes.NewReader([]byte(`{"invalid": json}`)))
		req = addTestContexts(req, userID, accountID.String())

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Ongeldige request body")
	})
}

func TestHandleGetCalendarEvents(t *testing.T) {
	accountID := uuid.New()
	userID := uuid.New()

	t.Run("Happy Path - Kan niet mocken", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleGetCalendarEvents(mockStore, testLogger)
		rr := httptest.NewRecorder()

		req, _ := http.NewRequest("GET", "/test", http.NoBody)
		req = addTestContexts(req, userID, accountID.String())

		// Mock de store call
		mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		mockStore.AssertExpectations(t)
	})

	t.Run("Error - Invalid Account ID", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleGetCalendarEvents(mockStore, testLogger)
		rr := httptest.NewRecorder()

		req, _ := http.NewRequest("GET", "/test", http.NoBody)
		req = addTestContexts(req, userID, "invalid-id")

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Ongeldig account ID")
	})
}

func TestHandleListCalendars(t *testing.T) {
	accountID := uuid.New()
	userID := uuid.New()

	// Arrange
	mockStore, testLogger := setupTestHandlers(t)
	handler := HandleListCalendars(mockStore, testLogger)
	rr := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/test", http.NoBody)
	req = addTestContexts(req, userID, accountID.String())

	// Mock de store call
	mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	mockStore.AssertExpectations(t)
}

func TestHandleGetAggregatedEvents(t *testing.T) {
	accountID := uuid.New()
	userID := uuid.New()

	// Request body
	reqBody := map[string]interface{}{
		"accounts": []map[string]string{
			{"accountId": accountID.String(), "calendarId": "primary"},
		},
	}
	body, _ := json.Marshal(reqBody)

	t.Run("Happy Path - Kan niet mocken", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleGetAggregatedEvents(mockStore, testLogger)
		rr := httptest.NewRecorder()

		req, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
		// Noot: Deze handler gebruikt geen chi URL params, alleen de user ID
		req = addTestContexts(req, userID, "")

		// Mock de store calls
		mockStore.On("GetConnectedAccountByID", mock.Anything, accountID).Return(domain.ConnectedAccount{
			ID:     accountID,
			UserID: userID, // Zorg dat de user eigenaar is
		}, nil)
		mockStore.On("GetValidTokenForAccount", mock.Anything, accountID).Return(&oauth2.Token{AccessToken: "test-token"}, nil)

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		mockStore.AssertExpectations(t)
	})

	t.Run("Error - No Auth User", func(t *testing.T) {
		// Arrange
		mockStore, testLogger := setupTestHandlers(t)
		handler := HandleGetAggregatedEvents(mockStore, testLogger)
		rr := httptest.NewRecorder()

		// Geen user ID in context
		req, _ := http.NewRequest("POST", "/test", http.NoBody)

		// Act
		handler.ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "missing or invalid user ID in context")
	})
}
