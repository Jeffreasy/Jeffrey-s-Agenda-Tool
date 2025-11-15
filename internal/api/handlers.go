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

// --- HELPER FUNCTIE (NIEUW) ---

// getCalendarClient initialiseert een Google Calendar client met token refresh
func (s *Server) getCalendarClient(ctx context.Context, account domain.ConnectedAccount) (*calendar.Service, error) {
	token := &oauth2.Token{
		AccessToken:  string(account.AccessToken),
		RefreshToken: string(account.RefreshToken),
		Expiry:       account.TokenExpiry,
		TokenType:    "Bearer",
	}

	// Refresh token als verlopen
	if !token.Valid() {
		newToken, err := s.googleOAuthConfig.TokenSource(ctx, token).Token()
		if err != nil {
			return nil, err
		}
		// Update token in DB
		updateParams := store.UpdateConnectedAccountTokenParams{
			ID:           account.ID,
			AccessToken:  []byte(newToken.AccessToken),
			RefreshToken: []byte(newToken.RefreshToken),
			TokenExpiry:  newToken.Expiry,
		}
		if err := s.store.UpdateConnectedAccountToken(ctx, updateParams); err != nil {
			return nil, err
		}
		token = newToken
	}

	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}
