package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"

	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const oauthStateCookieName = "oauthstate"

// generateJWT creert een nieuw JWT token voor een gebruiker
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

// HandleGoogleLogin starts the OAuth flow to Google.
func HandleGoogleLogin(oauthConfig *oauth2.Config, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil { // GOED
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon state niet genereren", log)
			return
		}
		state := base64.URLEncoding.EncodeToString(b)

		cookie := &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   60 * 10,
		}
		http.SetCookie(w, cookie)

		authURL := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleGoogleCallback handles the callback from Google after login.
func HandleGoogleCallback(storer store.Storer, oauthConfig *oauth2.Config, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Geen state cookie", log)
			return
		}
		if r.URL.Query().Get("state") != stateCookie.Value {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige state token", log)
			return
		}

		code := r.URL.Query().Get("code")
		token, err := oauthConfig.Exchange(ctx, code)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon code niet inwisselen: %s", err.Error()),
				log,
			)
			return
		}
		if token.RefreshToken == "" {
			common.WriteJSONError(
				w,
				http.StatusBadRequest,
				"Geen refresh token ontvangen. Probeer opnieuw.",
				log,
			)
			return
		}

		userInfo, err := getUserInfo(ctx, token)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon gebruikersinfo niet ophalen: %s", err.Error()),
				log,
			)
			return
		}

		user, err := storer.CreateUser(ctx, userInfo.Email, userInfo.Name)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon gebruiker niet aanmaken: %s", err.Error()),
				log,
			)
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
			Scopes:         oauthConfig.Scopes,
		}

		account, err := storer.UpsertConnectedAccount(ctx, params)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon account niet koppelen: %s", err.Error()),
				log,
			)
			return
		}

		log.Info(
			"Account gekoppeld",
			zap.String("account_id", account.ID.String()),
			zap.String("user_id", user.ID.String()),
		)

		jwtString, err := generateJWT(user.ID)
		if err != nil {
			common.WriteJSONError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Kon authenticatie-token niet genereren: %s", err.Error()),
				log,
			)
			return
		}

		redirectURL := fmt.Sprintf("%s/dashboard?token=%s", os.Getenv("CLIENT_BASE_URL"), jwtString)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

// getUserInfo haalt profielinfo op met een geldig token
func getUserInfo(ctx context.Context, token *oauth2.Token) (*oauth2v2.Userinfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	// --- AANGEPAST VOOR TESTBAARHEID ---
	opts := []option.ClientOption{option.WithHTTPClient(client)}

	// Als OAUTH2_USERINFO_URL is ingesteld (in een test), gebruik die
	if testEndpoint := os.Getenv("OAUTH2_USERINFO_URL"); testEndpoint != "" {
		opts = append(opts, option.WithEndpoint(testEndpoint))
	}
	// --- EINDE AANPASSING ---

	oauth2Service, err := oauth2v2.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// --- AANGEPAST: Gebruik het V2 Me pad ---
	// Dit zorgt ervoor dat het endpoint correct wordt samengevoegd
	userInfo, err := oauth2Service.Userinfo.V2.Me.Get().Do()
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}
