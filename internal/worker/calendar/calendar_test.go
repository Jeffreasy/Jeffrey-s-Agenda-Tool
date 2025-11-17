package calendar

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// --- Helpers ---

// setupCalendarTest stelt een processor, mock store, en een fake Google API server op.
func setupCalendarTest(t *testing.T, handler http.Handler) (*CalendarProcessor, *store.MockStore, *httptest.Server) {
	t.Helper()
	mockStore := new(store.MockStore)
	processor := NewCalendarProcessor(mockStore)

	// Maak de fake Google API server
	server := httptest.NewServer(handler)
	return processor, mockStore, server
}

// mockToken maakt een simpel token voor testing
func mockToken() *oauth2.Token {
	return &oauth2.Token{AccessToken: "fake-token"}
}

// --- Tests ---

// Test 1: Een event matcht een regel en maakt een nieuw event aan.
func TestCalendar_ProcessEvents_MatchAndCreate(t *testing.T) {
	// --- Arrange ---
	accountID := uuid.New()
	ruleID := uuid.New()
	triggerEventID := "trigger-event-id"
	createdEventID := "new-event-id"

	// 1. Definieer de regel die de store zal teruggeven
	triggerCond, _ := json.Marshal(domain.TriggerConditions{
		SummaryEquals: "Dienst",
	})
	actionParams, _ := json.Marshal(domain.ActionParams{
		OffsetMinutes: -60,
		NewEventTitle: "Reminder: Dienst",
		DurationMin:   5,
	})
	testRule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				BaseEntity: domain.BaseEntity{
					ID: ruleID,
				},
				ConnectedAccountID: accountID,
			},
			Name:              "Test Rule",
			IsActive:          true,
			TriggerConditions: triggerCond,
			ActionParams:      actionParams,
		},
	}

	// 2. Maak de Fake Google API Handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Fake Google API: Ontvangen request: %s %s", r.Method, r.URL.Path)

		// We verwachten 3 calls:
		// 1. GET .../events (List) -> de 'ProcessEvents' lijst
		// 2. GET .../events (List) -> de 'eventExists' check
		// 3. POST .../events (Insert) -> de 'Insert' call

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/events") {
			// Is dit de 'eventExists' check? Die heeft een specifieke timeMin/timeMax
			if r.URL.Query().Get("timeMin") == "2025-11-30T07:59:00Z" {
				t.Log("Fake Google API: Beantwoorden 'eventExists' check (geen events)")
				// Geef geen events terug, zodat de check faalt (event bestaat niet)
				listResp := calendar.Events{Items: []*calendar.Event{}}
				json.NewEncoder(w).Encode(listResp)
				return
			}

			// Dit is de hoofd 'List' call. Geef het trigger event terug.
			t.Log("Fake Google API: Beantwoorden hoofd 'List' call (1 event)")
			listResp := calendar.Events{
				Items: []*calendar.Event{
					{
						Id:      triggerEventID,
						Summary: "Dienst",
						Start:   &calendar.EventDateTime{DateTime: "2025-11-30T09:00:00Z"},
						End:     &calendar.EventDateTime{DateTime: "2025-11-30T17:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(listResp)
			return
		}

		if r.Method == "POST" && strings.Contains(r.URL.Path, "/events") {
			// Dit is de Insert call.
			t.Log("Fake Google API: Beantwoorden 'Insert' call")
			var newEvent calendar.Event
			err := json.NewDecoder(r.Body).Decode(&newEvent)
			require.NoError(t, err)

			// Assert dat de body van het nieuwe event correct is
			assert.Equal(t, "Reminder: Dienst", newEvent.Summary)
			assert.Equal(t, "2025-11-30T08:00:00Z", newEvent.Start.DateTime) // 1 uur ervoor
			assert.Equal(t, "2025-11-30T08:05:00Z", newEvent.End.DateTime)   // 5 min duur

			// Geef het aangemaakte event terug
			newEvent.Id = createdEventID
			json.NewEncoder(w).Encode(newEvent)
			return
		}

		// Noodgevallen
		t.Errorf("Onverwacht request naar Fake Google API: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	})

	// 3. Setup de test
	processor, mockStore, server := setupCalendarTest(t, handler)
	defer server.Close()

	// Override newService to use test server endpoint
	processor.newService = func(ctx context.Context, client *http.Client) (*calendar.Service, error) {
		return calendar.NewService(ctx, option.WithHTTPClient(client), option.WithEndpoint(server.URL))
	}

	testToken := mockToken()
	ctx := context.Background()

	// 4. Stel de mock store verwachtingen in
	mockStore.On("GetRulesForAccount", ctx, accountID).Return([]domain.AutomationRule{testRule}, nil).Once()
	mockStore.On("HasLogForTrigger", ctx, ruleID, triggerEventID).Return(false, nil).Once() // Nog niet gelogd

	// Verwacht dat de SUCCES log wordt aangemaakt
	mockStore.On("CreateAutomationLog", ctx, mock.MatchedBy(func(params store.CreateLogParams) bool {
		return params.Status == domain.LogSuccess &&
			*params.RuleID == ruleID &&
			strings.Contains(string(params.ActionDetails), createdEventID)
	})).Return(nil).Once()

	// --- Act ---
	err := processor.ProcessEvents(ctx, &domain.ConnectedAccount{ID: accountID}, testToken)

	// --- Assert ---
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

// Test 2: Event matcht, maar is al gelogd (deduplicatie).
func TestCalendar_ProcessEvents_AlreadyLogged(t *testing.T) {
	// --- Arrange ---
	accountID := uuid.New()
	ruleID := uuid.New()
	triggerEventID := "trigger-event-id"

	// 1. Definieer de regel
	triggerCond, _ := json.Marshal(domain.TriggerConditions{SummaryEquals: "Dienst"})
	testRule := domain.AutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			AccountEntity: domain.AccountEntity{
				BaseEntity: domain.BaseEntity{
					ID: ruleID,
				},
				ConnectedAccountID: accountID,
			},
			IsActive:          true,
			TriggerConditions: triggerCond,
			ActionParams:      json.RawMessage(`{}`),
		},
	}

	// 2. Fake Google API - geeft alleen het trigger event terug
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/events") {
			listResp := calendar.Events{
				Items: []*calendar.Event{
					{Id: triggerEventID, Summary: "Dienst", Start: &calendar.EventDateTime{DateTime: "2025-11-30T09:00:00Z"}},
				},
			}
			json.NewEncoder(w).Encode(listResp)
			return
		}
	})

	processor, mockStore, server := setupCalendarTest(t, handler)
	defer server.Close()

	// Override newService to use test server endpoint
	processor.newService = func(ctx context.Context, client *http.Client) (*calendar.Service, error) {
		return calendar.NewService(ctx, option.WithHTTPClient(client), option.WithEndpoint(server.URL))
	}

	testToken := mockToken()
	ctx := context.Background()

	// 3. Stel de mock store verwachtingen in
	mockStore.On("GetRulesForAccount", ctx, accountID).Return([]domain.AutomationRule{testRule}, nil).Once()

	// BELANGRIJK: De log bestaat al!
	mockStore.On("HasLogForTrigger", ctx, ruleID, triggerEventID).Return(true, nil).Once()

	// --- Act ---
	err := processor.ProcessEvents(ctx, &domain.ConnectedAccount{ID: accountID}, testToken)

	// --- Assert ---
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)

	// Verifieer dat we NOOIT een log hebben proberen te maken (omdat we skipten)
	mockStore.AssertNotCalled(t, "CreateAutomationLog")
}
