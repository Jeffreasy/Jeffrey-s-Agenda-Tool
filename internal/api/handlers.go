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

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// --- NIEUWE AUTH HANDLERS ---

const oauthStateCookieName = "oauthstate"

// handleGoogleLogin start de OAuth-flow
func (s *Server) handleGoogleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Genereer een veilige 'state' token (tegen CSRF)
		b := make([]byte, 32)
		rand.Read(b)
		state := base64.URLEncoding.EncodeToString(b)

		// 2. Sla de 'state' op in een cookie
		cookie := &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   60 * 10, // 10 minuten
		}
		http.SetCookie(w, cookie)

		// 3. Stuur de gebruiker door naar de Google consent pagina
		// 'Offline' en 'ApprovalForce' om altijd refresh_token te krijgen
		authURL := s.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// handleGoogleCallback is het endpoint dat Google aanroept na de login
func (s *Server) handleGoogleCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 1. Controleer de 'state' token uit de cookie
		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Geen state cookie")
			return
		}
		if r.URL.Query().Get("state") != stateCookie.Value {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldige state token")
			return
		}

		// 2. Haal de 'code' uit de URL en wissel hem in voor tokens
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

		// 3. Haal de profielinformatie van de gebruiker op
		userInfo, err := getUserInfo(ctx, token)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruikersinfo niet ophalen: %s", err.Error()))
			return
		}

		// 4. Vind of maak de gebruiker aan in onze DB
		user, err := s.store.CreateUser(ctx, userInfo.Email, userInfo.Name)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruiker niet aanmaken: %s", err.Error()))
			return
		}

		// 5. Sla de nieuwe tokens veilig op (upsert)
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

		// 6. Stuur de gebruiker terug naar de frontend
		http.Redirect(w, r, os.Getenv("CLIENT_BASE_URL")+"/dashboard?success=true", http.StatusSeeOther)
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

// --- CreateUser (Bestaande code) ---

type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) handleCreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Email == "" {
			WriteJSONError(w, http.StatusBadRequest, "Email is required")
			return
		}

		user, err := s.store.CreateUser(r.Context(), req.Email, req.Name)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Could not create user")
			return
		}

		WriteJSON(w, http.StatusCreated, user)
	}
}

// --- CreateConnectedAccount (Verwijderd, want overbodig na callback upsert) ---

// Voeg endpoint toe voor rules creatie (nieuw)
type CreateAutomationRuleRequest struct {
	ConnectedAccountID uuid.UUID       `json:"connected_account_id"`
	Name               string          `json:"name"`
	TriggerConditions  json.RawMessage `json:"trigger_conditions"`
	ActionParams       json.RawMessage `json:"action_params"`
}

func (s *Server) handleCreateAutomationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateAutomationRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		params := store.CreateAutomationRuleParams{
			ConnectedAccountID: req.ConnectedAccountID,
			Name:               req.Name,
			TriggerConditions:  req.TriggerConditions,
			ActionParams:       req.ActionParams,
		}

		rule, err := s.store.CreateAutomationRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Could not create rule")
			return
		}

		WriteJSON(w, http.StatusCreated, rule)
	}
}
