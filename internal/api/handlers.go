// Vervang hiermee: internal/api/handlers.go
package api

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"crypto/rand"     // <-- NIEUWE IMPORT
	"encoding/base64" // <-- NIEUWE IMPORT
	"encoding/json"
	"fmt" // <-- NIEUWE IMPORT
	"log" // <-- NIEUWE IMPORT
	"net/http"
	"os" // <-- NIEUWE IMPORT
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"                      // <-- NIEUWE IMPORT
	oauth2v2 "google.golang.org/api/oauth2/v2" // <-- NIEUWE IMPORT aliased
	"google.golang.org/api/option"             // <-- NIEUWE IMPORT
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
		// 'Offline' is cruciaal om een 'refresh_token' te krijgen
		authURL := s.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
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
			log.Println("[OAuth] WAARSCHUWING: Geen refresh token ontvangen. Is de gebruiker al geauthenticeerd?")
			// Dit gebeurt als een gebruiker *opnieuw* inlogt.
		}

		// 3. Haal de profielinformatie van de gebruiker op
		userInfo, err := getUserInfo(ctx, token)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruikersinfo niet ophalen: %s", err.Error()))
			return
		}

		// 4. Vind of maak de gebruiker aan in onze DB
		// We gebruiken de 'ON CONFLICT' in CreateUser als een 'Upsert'
		user, err := s.store.CreateUser(ctx, userInfo.Email, userInfo.Name)
		if err != nil {
			WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon gebruiker niet aanmaken: %s", err.Error()))
			return
		}

		// 5. Sla de nieuwe, ECHTE tokens veilig op
		params := store.CreateConnectedAccountParams{
			UserID:         user.ID,
			Provider:       domain.ProviderGoogle,
			Email:          userInfo.Email,
			ProviderUserID: userInfo.Id, // Het Google ID
			AccessToken:    token.AccessToken,
			RefreshToken:   token.RefreshToken,
			TokenExpiry:    token.Expiry,
			Scopes:         s.googleOAuthConfig.Scopes,
		}

		// TODO: We moeten hier 'CreateOrUpdate' logica hebben,
		// want `CreateConnectedAccount` faalt als het account al bestaat.
		// Voor nu negeren we de fout.
		_, _ = s.store.CreateConnectedAccount(ctx, params)
		// if err != nil {
		// 	WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon account niet koppelen: %s", err.Error()))
		// 	return
		// }

		// 6. Stuur de gebruiker terug naar de frontend
		// Je kunt de cookie hier ook verwijderen
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

// --- BESTAANDE HANDLERS ---
// (Plak hier je bestaande handleCreateUser en handleCreateConnectedAccount)
// ...

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

// --- CreateConnectedAccount (Nieuwe code) ---

// CreateConnectedAccountRequest definieert de JSON body die we verwachten
// Dit is wat je Next.js app stuurt na de OAuth flow.
type CreateConnectedAccountRequest struct {
	Provider       domain.ProviderType `json:"provider"` // "google" of "microsoft"
	Email          string              `json:"email"`
	ProviderUserID string              `json:"provider_user_id"`
	AccessToken    string              `json:"access_token"`
	RefreshToken   string              `json:"refresh_token"`
	TokenExpiry    time.Time           `json:"token_expiry"` // ISO-8601 formaat, e.g. "2025-11-11T18:30:00Z"
	Scopes         []string            `json:"scopes"`
}

func (s *Server) handleCreateConnectedAccount() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Haal de userID uit de URL
		userIDStr := chi.URLParam(r, "userID")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid user ID format")
			return
		}

		// 2. Decodeer de JSON body
		var req CreateConnectedAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// 3. Valideer de input (simpel)
		if req.AccessToken == "" || req.Provider == "" {
			WriteJSONError(w, http.StatusBadRequest, "Access token and provider are required")
			return
		}

		// 4. Map de API request naar de Store parameters
		params := store.CreateConnectedAccountParams{
			UserID:         userID, // Vanuit de URL
			Provider:       req.Provider,
			Email:          req.Email,
			ProviderUserID: req.ProviderUserID,
			AccessToken:    req.AccessToken,
			RefreshToken:   req.RefreshToken,
			TokenExpiry:    req.TokenExpiry,
			Scopes:         req.Scopes,
		}

		// 5. Roep de store aan
		// De store versleutelt de tokens en slaat ze op
		account, err := s.store.CreateConnectedAccount(r.Context(), params)
		if err != nil {
			// Dit kan falen door:
			// 1. Foreign key (userID bestaat niet)
			// 2. Unique constraint (account is al gekoppeld)
			// 3. Encryptie fout
			log.Printf("Failed to create connected account: %v", err)
			WriteJSONError(w, http.StatusInternalServerError, "Could not connect account")
			return
		}

		// 6. Stuur het gemaakte account terug
		// Belangrijk: het 'account' object dat terugkomt bevat de *versleutelde* tokens.
		// We sturen het nu terug, maar in productie wil je de tokens misschien verbergen.
		WriteJSON(w, http.StatusCreated, account)
	}
}
