package calendar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/api/calendar/v3"
)

const defaultCalendarID = "primary"

// HandleCreateEvent creates a new event in Google Calendar.
func HandleCreateEvent(store store.Storer, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", logger) // <-- AANGEPAST
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = defaultCalendarID
		}

		var req calendar.Event
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", logger) // <-- AANGEPAST
			return
		}

		ctx := r.Context()
		client, err := common.GetCalendarClient(ctx, store, accountID, logger)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				"Kon calendar client niet initialiseren",
				logger,
			)
			return
		}

		createdEvent, err := client.Events.Insert(calendarID, &req).Do()
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon event niet creren: %v", err),
				logger,
			)
			return
		}

		common.WriteJSON(w, http.StatusCreated, createdEvent, logger) // <-- AANGEPAST
	}
}

// HandleUpdateEvent updates an existing event.
func HandleUpdateEvent(store store.Storer, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		eventID := chi.URLParam(r, "eventId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", logger) // <-- AANGEPAST
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = defaultCalendarID
		}

		var req calendar.Event
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", logger) // <-- AANGEPAST
			return
		}

		ctx := r.Context()
		client, err := common.GetCalendarClient(ctx, store, accountID, logger)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				"Kon calendar client niet initialiseren",
				logger,
			)
			return
		}

		updatedEvent, err := client.Events.Update(calendarID, eventID, &req).Do()
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon event niet updaten: %v", err),
				logger,
			)
			return
		}

		common.WriteJSON(w, http.StatusOK, updatedEvent, logger) // <-- AANGEPAST
	}
}

// HandleDeleteEvent deletes an event.
func HandleDeleteEvent(store store.Storer, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		eventID := chi.URLParam(r, "eventId")
		calendarID := r.URL.Query().Get("calendarId") // Optioneel param voor secundaire calendar
		if calendarID == "" {
			calendarID = defaultCalendarID
		}

		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", logger) // <-- AANGEPAST
			return
		}

		ctx := r.Context()
		client, err := common.GetCalendarClient(ctx, store, accountID, logger)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				"Kon calendar client niet initialiseren",
				logger,
			)
			return
		}

		err = client.Events.Delete(calendarID, eventID).Do()
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon event niet verwijderen: %v", err),
				logger,
			)
			return
		}

		common.WriteJSON(w, http.StatusNoContent, nil, logger) // <-- AANGEPAST
	}
}

// HandleGetCalendarEvents retrieves events (optional calendarId param).
func HandleGetCalendarEvents(store store.Storer, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", logger) // <-- AANGEPAST
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Nieuw: Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = defaultCalendarID
		}

		// -----------------------------------------------------------------
		// HIER IS DE CORRECTIE (VERVANG JE OUDE timeMin/timeMax)
		// -----------------------------------------------------------------
		timeMinStr := r.URL.Query().Get("timeMin")
		if timeMinStr == "" {
			// Default op 'nu' als de frontend niets meegeeft
			timeMinStr = time.Now().Format(time.RFC3339)
		}

		timeMaxStr := r.URL.Query().Get("timeMax")
		if timeMaxStr == "" {
			// Default op 3 maanden vooruit als de frontend niets meegeeft
			timeMaxStr = time.Now().AddDate(0, 3, 0).Format(time.RFC3339)
		}
		// -----------------------------------------------------------------

		ctx := r.Context()
		userID, _ := common.GetUserIDFromContext(ctx) // Get user ID for logging

		client, err := common.GetCalendarClient(ctx, store, accountID, logger)
		if err != nil {
			logger.Error(
				"failed to initialize calendar client",
				zap.Error(err),
				zap.String("account_id", accountID.String()),
				zap.String("user_id", userID.String()),
				zap.String("calendar_id", calendarID),
				zap.String("component", "api"),
			)
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				"Kon calendar client niet initialiseren",
				logger,
			)
			return
		}

		start := time.Now()
		events, err := client.Events.List(calendarID).
			TimeMin(timeMinStr). // <-- Gebruik de variabele
			TimeMax(timeMaxStr). // <-- Gebruik de variabele
			SingleEvents(true).
			OrderBy("startTime").
			MaxResults(250). // <-- VOEG DIT LIMIET TOE
			Do()
		if err != nil {
			duration := time.Since(start)
			logger.Error(
				"failed to fetch calendar events",
				zap.Error(err),
				zap.String("account_id", accountID.String()),
				zap.String("user_id", userID.String()),
				zap.String("calendar_id", calendarID),
				zap.Int64("duration_ms", duration.Milliseconds()),
				zap.String("component", "api"),
			)
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon events niet ophalen: %v", err),
				logger,
			)
			return
		}

		duration := time.Since(start)
		logger.Info(
			"successfully fetched calendar events",
			zap.String("account_id", accountID.String()),
			zap.String("user_id", userID.String()),
			zap.String("calendar_id", calendarID),
			zap.Int("event_count", len(events.Items)),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.String("component", "api"),
		)

		common.WriteJSON(w, http.StatusOK, events.Items, logger) // <-- AANGEPAST
	}
}

// HandleListCalendars lists calendars for an account.
func HandleListCalendars(store store.Storer, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", logger) // <-- AANGEPAST
			return
		}

		ctx := r.Context()
		userID, _ := common.GetUserIDFromContext(ctx) // Get user ID for logging

		client, err := common.GetCalendarClient(ctx, store, accountID, logger)
		if err != nil {
			logger.Error(
				"failed to initialize calendar client",
				zap.Error(err),
				zap.String("account_id", accountID.String()),
				zap.String("user_id", userID.String()),
				zap.String("component", "api"),
			)
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				"Kon calendar client niet initialiseren",
				logger,
			)
			return
		}

		start := time.Now()
		calendars, err := client.CalendarList.List().Do()
		if err != nil {
			duration := time.Since(start)
			logger.Error(
				"failed to fetch calendars",
				zap.Error(err),
				zap.String("account_id", accountID.String()),
				zap.String("user_id", userID.String()),
				zap.Int64("duration_ms", duration.Milliseconds()),
				zap.String("component", "api"),
			)
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon calendars niet ophalen: %v", err),
				logger,
			)
			return
		}

		duration := time.Since(start)
		logger.Info(
			"successfully fetched calendars",
			zap.String("account_id", accountID.String()),
			zap.String("user_id", userID.String()),
			zap.Int("calendar_count", len(calendars.Items)),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.String("component", "api"),
		)

		common.WriteJSON(w, http.StatusOK, calendars.Items, logger) // <-- AANGEPAST
	}
}

// HandleGetAggregatedEvents retrieves events across accounts/calendars.
func HandleGetAggregatedEvents(store store.Storer, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), logger) // <-- AANGEPAST
			return
		}

		// Parse request body: lijst van {accountId, calendarId} pairs
		type AggRequest struct {
			Accounts []struct {
				AccountID  string `json:"accountId"`
				CalendarID string `json:"calendarId"`
			} `json:"accounts"`
		}
		var req AggRequest
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", logger) // <-- AANGEPAST
			return
		}

		timeMin := time.Now()
		timeMax := timeMin.AddDate(0, 3, 0)

		var allEvents []*calendar.Event
		ctx := r.Context()

		for _, acc := range req.Accounts {
			accountID, err := uuid.Parse(acc.AccountID)
			if err != nil {
				continue // Skip invalid
			}

			account, err := store.GetConnectedAccountByID(ctx, accountID)
			if err != nil || account.UserID != userID {
				continue // Skip not found or not owned
			}

			client, err := common.GetCalendarClient(ctx, store, accountID, logger) // <-- AANGEPAST
			if err != nil {
				continue
			}

			calID := acc.CalendarID
			if calID == "" {
				calID = defaultCalendarID
			}

			events, err := client.Events.List(calID).
				TimeMin(timeMin.Format(time.RFC3339)).
				TimeMax(timeMax.Format(time.RFC3339)).
				SingleEvents(true).
				OrderBy("startTime").
				Do()
			if err != nil {
				continue
			}

			allEvents = append(allEvents, events.Items...)
		}

		common.WriteJSON(w, http.StatusOK, allEvents, logger) // <-- AANGEPAST
	}
}
