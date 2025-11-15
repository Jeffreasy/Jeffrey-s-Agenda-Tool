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

	"github.com/go-chi/chi/v5" // NIEUWE IMPORT
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// --- AUTH HANDLERS ---

const oauthStateCookieName = "oauthstate"

// NIEUWE HELPER: Haalt de user ID op die door de middleware in de context is gezet
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

// --- handleCreateAutomationRule (Bestaande code) ---

type CreateAutomationRuleRequest struct {
	ConnectedAccountID uuid.UUID       `json:"connected_account_id"`
	Name               string          `json:"name"`
	TriggerConditions  json.RawMessage `json:"trigger_conditions"`
	ActionParams       json.RawMessage `json:"action_params"`
}

func (s *Server) handleCreateAutomationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker (uit de context)
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Decodeer de request body
		var req CreateAutomationRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// 3. (KRITIEK) Controleer of deze gebruiker wel de eigenaar is van het account
		if err := s.store.VerifyAccountOwnership(r.Context(), req.ConnectedAccountID, userID); err != nil {
			log.Printf("Forbidden access attempt: User %s tried to access account %s", userID, req.ConnectedAccountID)
			WriteJSONError(w, http.StatusForbidden, "Je hebt geen toegang tot dit account")
			return
		}

		// 4. Ga door met het aanmaken van de regel (nu veilig)
		params := store.CreateAutomationRuleParams{
			ConnectedAccountID: req.ConnectedAccountID, // We weten nu dat dit ID veilig is
			Name:               req.Name,
			TriggerConditions:  req.TriggerConditions,
			ActionParams:       req.ActionParams,
		}

		rule, err := s.store.CreateAutomationRule(r.Context(), params)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon regel niet aanmaken")
			return
		}

		WriteJSON(w, http.StatusCreated, rule)
	}
}

// --- NIEUWE READ HANDLERS ---

// handleGetConnectedAccounts haalt alle accounts op voor de ingelogde gebruiker
func (s *Server) handleGetConnectedAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Haal de accounts op uit de store
		accounts, err := s.store.GetAccountsForUser(r.Context(), userID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon accounts niet ophalen")
			return
		}

		// 3. (BELANGRIJK) Verberg de versleutelde tokens voordat we ze terugsturen.
		// We willen nooit access/refresh tokens (zelfs versleuteld) naar de client sturen.
		type PublicAccount struct {
			ID             uuid.UUID            `json:"id"`
			UserID         uuid.UUID            `json:"user_id"`
			Provider       domain.ProviderType  `json:"provider"`
			Email          string               `json:"email"`
			Status         domain.AccountStatus `json:"status"`
			ProviderUserID string               `json:"provider_user_id"`
			CreatedAt      time.Time            `json:"created_at"`
		}

		publicAccounts := make([]PublicAccount, len(accounts))
		for i, acc := range accounts {
			publicAccounts[i] = PublicAccount{
				ID:             acc.ID,
				UserID:         acc.UserID,
				Provider:       acc.Provider,
				Email:          acc.Email,
				Status:         acc.Status,
				ProviderUserID: acc.ProviderUserID,
				CreatedAt:      acc.CreatedAt,
			}
		}

		WriteJSON(w, http.StatusOK, publicAccounts)
	}
}

// handleGetAutomationRules haalt alle regels op voor een specifiek, eigendom
func (s *Server) handleGetAutomationRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Haal de 'accountID' op uit de URL path (e.g. /accounts/uuid-hier/rules)
		accountIDStr := chi.URLParam(r, "accountID")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID formaat")
			return
		}

		// 3. (KRITIEK) Controleer eigendom
		if err := s.store.VerifyAccountOwnership(r.Context(), accountID, userID); err != nil {
			log.Printf("Forbidden access attempt: User %s tried to access rules for account %s", userID, accountID)
			WriteJSONError(w, http.StatusForbidden, "Je hebt geen toegang tot dit account")
			return
		}

		// 4. Haal de regels op (nu veilig)
		rules, err := s.store.GetRulesForAccount(r.Context(), accountID)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon regels niet ophalen")
			return
		}

		// De 'rules' struct bevat geen geheimen, dus we kunnen het direct teruggeven.
		WriteJSON(w, http.StatusOK, rules)
	}
}

// --- NIEUWE HANDLER (Feature 1) ---

// PublicAutomationLog is een struct die we veilig kunnen teruggeven aan de client.
type PublicAutomationLog struct {
	ID             int64                      `json:"id"`
	Timestamp      time.Time                  `json:"timestamp"`
	Status         domain.AutomationLogStatus `json:"status"`
	TriggerDetails json.RawMessage            `json:"trigger_details"`
	ActionDetails  json.RawMessage            `json:"action_details"`
	ErrorMessage   string                     `json:"error_message"`
}

// handleGetAutomationLogs haalt de recente logs op voor een account.
func (s *Server) handleGetAutomationLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Haal de 'accountID' op uit de URL path
		accountIDStr := chi.URLParam(r, "accountID")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID formaat")
			return
		}

		// 3. (KRITIEK) Controleer eigendom
		if err := s.store.VerifyAccountOwnership(r.Context(), accountID, userID); err != nil {
			log.Printf("Forbidden access attempt: User %s tried to access logs for account %s", userID, accountID)
			WriteJSONError(w, http.StatusForbidden, "Je hebt geen toegang tot dit account")
			return
		}

		// 4. Haal de logs op (met een limiet)
		logs, err := s.store.GetLogsForAccount(r.Context(), accountID, 20) // Limiet op 20
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon logs niet ophalen")
			return
		}

		// 5. Converteer naar publieke struct (verbergt interne DB details)
		publicLogs := make([]PublicAutomationLog, len(logs))
		for i, log := range logs {
			publicLogs[i] = PublicAutomationLog{
				ID:             log.ID,
				Timestamp:      log.Timestamp,
				Status:         log.Status,
				TriggerDetails: log.TriggerDetails, // Is al []byte/json.RawMessage
				ActionDetails:  log.ActionDetails,
				ErrorMessage:   log.ErrorMessage,
			}
		}

		WriteJSON(w, http.StatusOK, publicLogs)
	}
}

// --- NIEUWE HANDLER (Feature 2) ---

// handleDeleteAutomationRule verwijdert een regel.
func (s *Server) handleDeleteAutomationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de ID op van de GEAUTHENTICEERDE gebruiker
		userID, err := getUserIDFromContext(r.Context())
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Niet geauthenticeerd")
			return
		}

		// 2. Haal de 'ruleID' op uit de URL path
		ruleIDStr := chi.URLParam(r, "ruleID")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Ongeldig regel ID formaat")
			return
		}

		// 3. (KRITIEK) Controleer eigendom
		if err := s.store.VerifyRuleOwnership(r.Context(), ruleID, userID); err != nil {
			log.Printf("Forbidden access attempt: User %s tried to delete rule %s", userID, ruleID)
			WriteJSONError(w, http.StatusForbidden, "Je hebt geen toegang tot deze regel")
			return
		}

		// 4. Verwijder de regel
		if err := s.store.DeleteRule(r.Context(), ruleID); err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "Kon regel niet verwijderen")
			return
		}

		w.WriteHeader(http.StatusNoContent) // Stuur 204 No Content terug
	}
}
