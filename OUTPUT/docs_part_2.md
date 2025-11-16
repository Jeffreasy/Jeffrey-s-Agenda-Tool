# Documentation Part 2

Generated at: 2025-11-15T20:45:21+01:00

## internal\api\handlers.go

```go
package api

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// --- AUTH HANDLERS (Bestaande code) ---

const oauthStateCookieName = "oauthstate"

// HELPER: Haalt de user ID op die door de middleware in de context is gezet
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(userContextKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("missing or invalid user ID in context")
	}
	return userID, nil
}

// generateJWT creëert een nieuw JWT token voor een gebruiker
func generateJWT(userID uuid.UUID) (string, error) {
	jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))
	if len(jwtKey) == 0 {
		return "", fmt.Errorf("JWT_SECRET_KEY is niet ingesteld")
	}

	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"iss":     "agenda-automator-api",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 dagen geldig
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", fmt.Errorf("kon token niet ondertekenen: %w", err)
	}

	return tokenString, nil
}

// handleGoogleLogin start de OAuth-flow
func (s *Server) handleGoogleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 32)
		rand.Read(b)
		state := base64.URLEncoding.EncodeToString(b)

		cookie := &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   60 * 10,
		}
		http.SetCookie(w, cookie)

		authURL := s.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// handleGoogleCallback is het endpoint dat Google aanroept na de login
func (s *Server) handleGoogleCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Geen state cookie")
			return
		}
		if r.URL.Query().Get("state") != stateCookie.Value {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige state token")
			return
		}

		code := r.URL.Query().Get("code")
		token, err := s.googleOAuthConfig.Exchange(ctx, code)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon code niet inwisselen: %s", err.Error()))
			return
		}
		if token.RefreshToken == "" {
			WriteJSONError(w, http.StatusBadRequest, "Geen refresh token ontvangen. Probeer opnieuw.")
			return
		}

		userInfo, err := getUserInfo(ctx, token)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruikersinfo niet ophalen: %s", err.Error()))
			return
		}

		user, err := s.store.CreateUser(ctx, userInfo.Email, userInfo.Name)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruiker niet aanmaken: %s", err.Error()))
			return
		}

		params := store.UpsertConnectedAccountParams{
			UserID:         user.ID,
			Provider:       domain.ProviderGoogle,
			Email:          userInfo.Email,
			ProviderUserID: userInfo.Id,
			AccessToken:    token.AccessToken,
			RefreshToken:   token.RefreshToken,
			TokenExpiry:    token.Expiry,
			Scopes:         s.googleOAuthConfig.Scopes,
		}

		account, err := s.store.UpsertConnectedAccount(ctx, params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon account niet koppelen: %s", err.Error()))
			return
		}

		log.Printf("Account %s gekoppeld voor user %s", account.ID, user.ID)

		jwtString, err := generateJWT(user.ID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon authenticatie-token niet genereren: %s", err.Error()))
			return
		}

		redirectURL := fmt.Sprintf("%s/dashboard?token=%s", os.Getenv("CLIENT_BASE_URL"), jwtString)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

// getUserInfo haalt profielinfo op met een geldig token
func getUserInfo(ctx context.Context, token *oauth2.Token) (*oauth2v2.Userinfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	oauth2Service, err := oauth2v2.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

// --- USER HANDLERS ---

// handleGetMe haalt de gegevens op van de ingelogde gebruiker.
func (s *Server) handleGetMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		user, err := s.store.GetUserByID(r.Context(), userID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon gebruiker niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, user)
	}
}

// --- ACCOUNT HANDLERS ---

// handleGetConnectedAccounts haalt alle gekoppelde accounts op voor de gebruiker.
func (s *Server) handleGetConnectedAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		accounts, err := s.store.GetAccountsForUser(r.Context(), userID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon accounts niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, accounts)
	}
}

// handleDeleteConnectedAccount verwijdert een gekoppeld account.
func (s *Server) handleDeleteConnectedAccount() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		err = s.store.DeleteConnectedAccount(r.Context(), accountID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon account niet verwijderen")
			return
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// --- RULE HANDLERS ---

// handleCreateRule creëert een nieuwe automation rule.
func (s *Server) handleCreateRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		var req domain.AutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		params := store.CreateAutomationRuleParams{
			ConnectedAccountID: accountID,
			Name:               req.Name,
			TriggerConditions:  req.TriggerConditions,
			ActionParams:       req.ActionParams,
		}

		rule, err := s.store.CreateAutomationRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet creëren")
			return
		}

		WriteJSON(w, http.StatusCreated, rule)
	}
}

// handleGetRules haalt alle rules op voor een account.
func (s *Server) handleGetRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		rules, err := s.store.GetRulesForAccount(r.Context(), accountID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rules niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, rules)
	}
}

// handleUpdateRule update een bestaande rule.
func (s *Server) handleUpdateRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		rule, err := s.store.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden")
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule")
			return
		}

		var req domain.AutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		params := store.UpdateRuleParams{
			RuleID:            ruleID,
			Name:              req.Name,
			TriggerConditions: req.TriggerConditions,
			ActionParams:      req.ActionParams,
		}

		updatedRule, err := s.store.UpdateRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet updaten")
			return
		}

		WriteJSON(w, http.StatusOK, updatedRule)
	}
}

// handleDeleteRule verwijdert een rule.
func (s *Server) handleDeleteRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		rule, err := s.store.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden")
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule")
			return
		}

		err = s.store.DeleteRule(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet verwijderen")
			return
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// handleToggleRule togglet de active status van een rule.
func (s *Server) handleToggleRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		rule, err := s.store.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden")
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule")
			return
		}

		updatedRule, err := s.store.ToggleRuleStatus(r.Context(), ruleID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon rule status niet togglen")
			return
		}

		WriteJSON(w, http.StatusOK, updatedRule)
	}
}

// --- LOG HANDLERS ---

// handleGetAutomationLogs haalt logs op voor een account.
func (s *Server) handleGetAutomationLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		account, err := s.store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		limit := 50 // Default limit
		logs, err := s.store.GetLogsForAccount(r.Context(), accountID, limit)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon logs niet ophalen")
			return
		}

		WriteJSON(w, http.StatusOK, logs)
	}
}

// --- NIEUWE HANDLERS VOOR EVENT CRUD ---

// handleCreateEvent creëert een nieuw event in Google Calendar
func (s *Server) handleCreateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = "primary"
		}

		var req calendar.Event
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		createdEvent, err := client.Events.Insert(calendarID, &req).Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon event niet creëren: %v", err))
			return
		}

		WriteJSON(w, http.StatusCreated, createdEvent)
	}
}

// handleUpdateEvent update een bestaand event
func (s *Server) handleUpdateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		eventID := chi.URLParam(r, "eventId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = "primary"
		}

		var req calendar.Event
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
			return
		}

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		updatedEvent, err := client.Events.Update(calendarID, eventID, &req).Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon event niet updaten: %v", err))
			return
		}

		WriteJSON(w, http.StatusOK, updatedEvent)
	}
}

// handleDeleteEvent verwijdert een event
func (s *Server) handleDeleteEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		eventID := chi.URLParam(r, "eventId")
		calendarID := r.URL.Query().Get("calendarId") // Optioneel param voor secundaire calendar
		if calendarID == "" {
			calendarID = "primary"
		}

		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		ctx := r.Context()
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		err = client.Events.Delete(calendarID, eventID).Do()
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon event niet verwijderen: %v", err))
			return
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// --- BIJGEWERKTE HANDLER VOOR EVENT FETCH (MET MULTI-CALENDAR SUPPORT) ---

// handleGetCalendarEvents haalt events op (nu met optionele calendarId param)
func (s *Server) handleGetCalendarEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID")
			return
		}

		calendarID := r.URL.Query().Get("calendarId") // Nieuw: Ondersteun secundaire calendars
		if calendarID == "" {
			calendarID = "primary"
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
		account, err := s.store.GetConnectedAccountByID(ctx, accountID)
		if err != nil {
			WriteJSONError(w, http.StatusNotFound, "Account niet gevonden")
			return
		}

		client, err := s.getCalendarClient(ctx, account)
		if err != nil {
			log.Printf("HANDLER ERROR [getCalendarClient]: %v", err)
			WriteJSONError(w, http.StatusInternalServerError, "Kon calendar client niet initialiseren")
			return
		}

		events, err := client.Events.List(calendarID).
			TimeMin(timeMinStr). // <-- Gebruik de variabele
			TimeMax(timeMaxStr). // <-- Gebruik de variabele
			SingleEvents(true).
			OrderBy("startTime").
			MaxResults(250). // <-- VOEG DIT LIMIET TOE
			Do()
		if err != nil {
			log.Printf("HANDLER ERROR [client.Events.List]: %v", err)
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon events niet ophalen: %v", err))
			return
		}

		WriteJSON(w, http.StatusOK, events.Items)
	}
}

// --- NIEUWE HANDLER VOOR AGGREGATED EVENTS ---

// handleGetAggregatedEvents haalt events op van meerdere accounts/calendars
func (s *Server) handleGetAggregatedEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, err.Error())
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body")
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

			account, err := s.store.GetConnectedAccountByID(ctx, accountID)
			if err != nil || account.UserID != userID {
				continue // Skip not found or not owned
			}

			client, err := s.getCalendarClient(ctx, account)
			if err != nil {
				continue
			}

			calID := acc.CalendarID
			if calID == "" {
				calID = "primary"
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

		WriteJSON(w, http.StatusOK, allEvents)
	}
}

// --- HEALTH CHECK HANDLER ---

// handleHealth checks if the API server is running and healthy.
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- HELPER FUNCTIE (NIEUW) ---

// getCalendarClient initialiseert een Google Calendar client met token refresh
func (s *Server) getCalendarClient(ctx context.Context, account domain.ConnectedAccount) (*calendar.Service, error) {

	// BELANGRIJK: Gebruik context.Background() voor externe calls,
	// NIET de 'ctx' van de request, om header-vervuiling te voorkomen.
	cleanCtx := context.Background()

	// 1. Haal het token op (deze functie gebruikt de DB context 'ctx', maar 'cleanCtx' voor de refresh)
	token, err := s.store.GetValidTokenForAccount(ctx, account.ID)
	if err != nil {
		log.Printf("HANDLER ERROR [getCalendarClient]: %v", err)
		return nil, fmt.Errorf("kon geen geldig token voor account ophalen: %w", err)
	}

	// 2. Maak de client en service aan met de schone context
	client := oauth2.NewClient(cleanCtx, oauth2.StaticTokenSource(token))

	return calendar.NewService(cleanCtx, option.WithHTTPClient(client))
}

```

## internal\domain\models.go

```go
package domain

import (
	"encoding/json" // Zorg dat deze import er is
	"time"

	"github.com/google/uuid"
)

// --- ENUM Types ---
type ProviderType string

const (
	ProviderGoogle    ProviderType = "google"
	ProviderMicrosoft ProviderType = "microsoft"
)

type AccountStatus string

const (
	StatusActive  AccountStatus = "active"
	StatusRevoked AccountStatus = "revoked"
	StatusError   AccountStatus = "error"
	StatusPaused  AccountStatus = "paused"
)

type AutomationLogStatus string

const (
	LogPending AutomationLogStatus = "pending"
	LogSuccess AutomationLogStatus = "success"
	LogFailure AutomationLogStatus = "failure"
	LogSkipped AutomationLogStatus = "skipped"
)

// --- Tabel Structs (met JSON tags) ---

type User struct {
	ID        uuid.UUID `db:"id"        json:"id"`
	Email     string    `db:"email"     json:"email"`
	Name      *string   `db:"name"      json:"name,omitempty"` // AANGEPAST: van 'string' naar '*string'
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type ConnectedAccount struct {
	ID             uuid.UUID     `db:"id"                 json:"id"`
	UserID         uuid.UUID     `db:"user_id"            json:"user_id"`
	Provider       ProviderType  `db:"provider"           json:"provider"`
	Email          string        `db:"email"              json:"email"`
	ProviderUserID string        `db:"provider_user_id"   json:"provider_user_id"`
	AccessToken    []byte        `db:"access_token"       json:"-"`
	RefreshToken   []byte        `db:"refresh_token"      json:"-"`
	TokenExpiry    time.Time     `db:"token_expiry"       json:"token_expiry"`
	Scopes         []string      `db:"scopes"             json:"scopes"`
	Status         AccountStatus `db:"status"             json:"status"`
	CreatedAt      time.Time     `db:"created_at"         json:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"         json:"updated_at"`
	LastChecked    *time.Time    `db:"last_checked"       json:"last_checked"`
}

type AutomationRule struct {
	ID                 uuid.UUID       `db:"id"                     json:"id"`
	ConnectedAccountID uuid.UUID       `db:"connected_account_id"   json:"connected_account_id"`
	Name               string          `db:"name"                   json:"name"`
	IsActive           bool            `db:"is_active"              json:"is_active"`
	TriggerConditions  json.RawMessage `db:"trigger_conditions"     json:"trigger_conditions"`
	ActionParams       json.RawMessage `db:"action_params"          json:"action_params"`
	CreatedAt          time.Time       `db:"created_at"             json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at"             json:"updated_at"`
}

type AutomationLog struct {
	ID                 int64               `db:"id"                     json:"id"`
	ConnectedAccountID uuid.UUID           `db:"connected_account_id"   json:"connected_account_id"`
	RuleID             uuid.UUID           `db:"rule_id"                json:"rule_id"`
	Timestamp          time.Time           `db:"timestamp"              json:"timestamp"`
	Status             AutomationLogStatus `db:"status"                 json:"status"`
	TriggerDetails     json.RawMessage     `db:"trigger_details"        json:"trigger_details"`
	ActionDetails      json.RawMessage     `db:"action_details"         json:"action_details"`
	ErrorMessage       string              `db:"error_message"          json:"error_message"`
}

// ... rest van het bestand (TriggerConditions, ActionParams, etc.) ...
// (Deze hoeven niet aangepast te worden)
type TriggerConditions struct {
	SummaryEquals    string   `json:"summary_equals,omitempty"`
	SummaryContains  []string `json:"summary_contains,omitempty"`
	LocationContains []string `json:"location_contains,omitempty"`
}

type ActionParams struct {
	OffsetMinutes int    `json:"offset_minutes"`
	NewEventTitle string `json:"new_event_title"`
	DurationMin   int    `json:"duration_min"`
}

type TriggerLogDetails struct {
	GoogleEventID  string    `json:"google_event_id"`
	TriggerSummary string    `json:"trigger_summary"`
	TriggerTime    time.Time `json:"trigger_time"`
}

type ActionLogDetails struct {
	CreatedEventID      string    `json:"created_event_id"`
	CreatedEventSummary string    `json:"created_event_summary"`
	ReminderTime        time.Time `json:"reminder_time"`
}

type Event struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	CalendarId  string    `json:"calendarId"`
}

```

## cmd\server\main.go

```go
package main

import (
	"context"
	// _ "embed" // <-- VERWIJDERD
	"log"
	"net/http"
	"os"
	"strings"

	// Interne packages
	"agenda-automator-api/internal/api"
	"agenda-automator-api/internal/database"
	"agenda-automator-api/internal/store"
	"agenda-automator-api/internal/worker"

	"agenda-automator-api/db/migrations" // <-- TOEGEVOEGD

	// Externe packages
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VERWIJDERD:
// //go:embed"../db/migrations/000001_initial_schema.up.sql"
// var migrationFile string

func main() {
	// 1. Laad configuratie (.env)
	_ = godotenv.Load()

	// 2. Maak verbinding met de Database
	pool, err := database.ConnectDB()
	if err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}
	defer pool.Close()

	// -----------------------------------------------------
	// AANGEPAST: Stap 2.5 - Voer migraties uit
	run := os.Getenv("RUN_MIGRATIONS")
	if strings.ToLower(run) == "true" {
		log.Println("Running database migrations...")
		// Gebruik de variabele uit de 'migrations' package:
		if _, err := pool.Exec(context.Background(), migrations.InitialSchemaUp); err != nil {
			log.Fatalf("Database migrations failed: %v", err)
		}
		log.Println("Database migrations applied successfully.")
	} else {
		log.Println("Skipping migrations (RUN_MIGRATIONS is not 'true')")
	}
	// -----------------------------------------------------

	// 3. Initialiseer de Gedeelde OAuth2 Config
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	redirectURL := os.Getenv("OAUTH_REDIRECT_URL") // e.g. http://localhost:8080/api/v1/auth/google/callback

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Fatalf("GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET, of OAUTH_REDIRECT_URL is niet ingesteld in .env")
	}

	googleOAuthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"https://www.googleapis.com/auth/calendar.events",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}

	// 4. Initialiseer de 'Store' Laag (AANGEPAST)
	// De store heeft nu de oauth config nodig om zelf tokens te verversen.
	dbStore := store.NewStore(pool, googleOAuthConfig)

	// 5. Initialiseer de Worker (geef de store mee)
	appWorker, err := worker.NewWorker(dbStore)
	if err != nil {
		log.Fatalf("Could not initialize worker: %v", err)
	}

	// 6. Start de Worker in de achtergrond
	appWorker.Start()

	// 7. Initialiseer de API Server (geef de store en config mee)
	apiServer := api.NewServer(dbStore, googleOAuthConfig)

	// 8. Start de HTTP Server (op de voorgrond)
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080" // Default poort
	}

	log.Printf("Application starting API server on port %s...", port)

	if err := http.ListenAndServe(":"+port, apiServer.Router); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}

```

## go.mod

```go-mod
module agenda-automator-api

go 1.24.0

toolchain go1.24.10

require (
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/cors v1.2.2
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/joho/godotenv v1.5.1
	golang.org/x/oauth2 v0.33.0
	google.golang.org/api v0.256.0
)

require (
	cloud.google.com/go/auth v0.17.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.7 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251103181224-f26f9409b101 // indirect
	google.golang.org/grpc v1.76.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

```

## .env.example

```bash
//---------------------------------------------------
// 1. APPLICATIE CONFIGURATIE
//---------------------------------------------------
APP_ENV=development

API_PORT=8080

//---------------------------------------------------
// 2. DATABASE (POSTGRES)
//---------------------------------------------------
POSTGRES_USER=postgres
POSTGRES_PASSWORD=Bootje12
POSTGRES_DB=agenda_automator
POSTGRES_HOST=localhost
POSTGRES_PORT=5433

DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

//---------------------------------------------------
// 3. CACHE (REDIS)
//---------------------------------------------------
// Verwijderd, want niet gebruikt

//---------------------------------------------------
// 4. BEVEILIGING & ENCRYPTIE
//---------------------------------------------------
ENCRYPTION_KEY="IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea"

//---------------------------------------------------
// 5. OAUTH CLIENTS (Google)
//---------------------------------------------------
CLIENT_BASE_URL="http://localhost:3000"

OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/google/callback"

GOOGLE_OAUTH_CLIENT_ID="YOUR-CLIENT-ID-HERE.apps.googleusercontent.com"
GOOGLE_OAUTH_CLIENT_SECRET="YOUR-CLIENT-SECRET-HERE"

// NIEUW: Voor dynamic CORS
ALLOWED_ORIGINS=http://localhost:3000,https://prod.com
```

## db\migrations\000001_initial_schema.down.sql

```sql
-- Rollback initial schema migration

DROP INDEX IF EXISTS idx_automation_logs_rule_id;
DROP INDEX IF EXISTS idx_automation_logs_account_id_timestamp;
DROP TABLE IF EXISTS automation_logs;

DROP INDEX IF EXISTS idx_automation_rules_account_id;
DROP TABLE IF EXISTS automation_rules;

DROP INDEX IF EXISTS idx_connected_accounts_user_id;
DROP TABLE IF EXISTS connected_accounts;

DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS automation_log_status;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS provider_type;
```

