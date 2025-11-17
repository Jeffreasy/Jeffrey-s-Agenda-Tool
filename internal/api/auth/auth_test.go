package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
)

//
// DEZE TESTS WAREN AL AANWEZIG (EN GOED)
//

func TestHandleGoogleLogin(t *testing.T) {
	testLogger := zap.NewNop()
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/api/v1/auth/google/callback",
		Scopes:       []string{"email", "profile"},
		Endpoint:     oauth2.Endpoint{}, // Will be set by the actual config
	}
	req, err := http.NewRequest("GET", "/api/v1/auth/google/login", http.NoBody)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := HandleGoogleLogin(oauthConfig, testLogger)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
	cookies := rr.Result().Cookies()
	var stateCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == oauthStateCookieName {
			stateCookie = cookie
			break
		}
	}
	assert.NotNil(t, stateCookie, "State cookie should be set")
	assert.NotEmpty(t, stateCookie.Value, "State cookie should have a value")
	assert.True(t, stateCookie.HttpOnly, "State cookie should be HttpOnly")
	assert.Equal(t, "/", stateCookie.Path, "State cookie should have correct path")
	assert.Greater(t, stateCookie.MaxAge, 0, "State cookie should have MaxAge set")
	location := rr.Header().Get("Location")
	assert.NotEmpty(t, location, "Location header should be set")
}

func TestGenerateJWT(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "test-secret-key")
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	token, err := generateJWT(userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	parts := strings.Split(token, ".")
	assert.Equal(t, 3, len(parts), "JWT should have 3 parts")
}

func TestGenerateJWT_NoSecret(t *testing.T) {
	t.Setenv("JWT_SECRET_KEY", "")
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	token, err := generateJWT(userID)
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "JWT_SECRET_KEY is niet ingesteld")
}

//
// --- NIEUWE TESTS VOOR HANDLEGOOGLECALLBACK ---
//

// setupCallbackTest is een helper
func setupCallbackTest(t *testing.T) (*httptest.Server, *store.MockStore, *zap.Logger, *oauth2.Config) {
	// 1. Maak een mock Google API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route op basis van het pad
		switch r.URL.Path {
		case "/token": // Token Exchange
			if r.FormValue("code") == "bad_code" {
				http.Error(w, "invalid_grant", http.StatusBadRequest)
				return
			}
			if r.FormValue("code") == "no_refresh" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(oauth2.Token{
					AccessToken:  "test-access-token",
					RefreshToken: "", // Geen refresh token
					Expiry:       time.Now().Add(1 * time.Hour),
				})
				return
			}
			// Voor de GetUserInfoFails test
			if r.FormValue("code") == "wrong_access_token_code" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(oauth2.Token{
					AccessToken:  "wrong-access-token",
					RefreshToken: "test-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(oauth2.Token{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				Expiry:       time.Now().Add(1 * time.Hour),
			})
		case "/userinfo/v2/me": // User Info
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer test-access-token" {
				http.Error(w, "invalid_token", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(oauth2v2.Userinfo{
				Id:    "google-user-123",
				Email: "test@example.com",
				Name:  "Test User",
			})
		default:
			http.NotFound(w, r)
		}
	}))

	// 2. Maak de mock Storer
	mockStore := new(store.MockStore)

	// 3. Maak een test logger
	testLogger := zap.NewNop()

	// 4. Maak een oauthConfig die naar de mock server wijst
	oauthConfig := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/callback",
		Scopes:       []string{"email", "profile", "calendar", "gmail.modify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  mockServer.URL + "/auth",
			TokenURL: mockServer.URL + "/token", // Wijst naar onze mock server
		},
	}
	t.Setenv("OAUTH2_USERINFO_URL", mockServer.URL)

	// Sluit de server als de test klaar is
	t.Cleanup(func() {
		mockServer.Close()
	})

	return mockServer, mockStore, testLogger, oauthConfig
}

func TestHandleGoogleCallback_HappyPath(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	t.Setenv("JWT_SECRET_KEY", "een-zeer-geheim-geheim-voor-jwt")
	t.Setenv("CLIENT_BASE_URL", "http://mijnfrontend.com")

	testUserID := uuid.New()
	testAccountID := uuid.New()

	mockStore.On("CreateUser", mock.Anything, "test@example.com", "Test User").
		Return(domain.User{BaseEntity: domain.BaseEntity{ID: testUserID}, Email: "test@example.com", Name: stringPtr("Test User")}, nil).
		Once()

	mockStore.On("UpsertConnectedAccount", mock.Anything, mock.MatchedBy(func(params store.UpsertConnectedAccountParams) bool {
		return params.UserID == testUserID &&
			params.Email == "test@example.com" &&
			params.ProviderUserID == "google-user-123" &&
			params.RefreshToken == "test-refresh-token"
	})).Return(domain.ConnectedAccount{ID: testAccountID, UserID: testUserID, Email: "test@example.com"}, nil).
		Once()

	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=valid_code&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{
		Name:  oauthStateCookieName,
		Value: "test_state",
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code, "Moet redirecten (303)")
	redirectURL := rr.Header().Get("Location")
	assert.True(t, strings.HasPrefix(redirectURL, "http://mijnfrontend.com/dashboard?token="), "Moet redirecten naar de frontend met een token")
	mockStore.AssertExpectations(t)
}

func TestHandleGoogleCallback_NoStateCookie(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=valid_code&state=test_state", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Geen state cookie")
}

func TestHandleGoogleCallback_InvalidState(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=valid_code&state=wrong_state", http.NoBody)
	req.AddCookie(&http.Cookie{
		Name:  oauthStateCookieName,
		Value: "correct_state",
	})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Ongeldige state token")
}

func TestHandleGoogleCallback_ExchangeFails(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=bad_code&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "test_state"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Kon code niet inwisselen")
}

func TestHandleGoogleCallback_NoRefreshToken(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=no_refresh&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "test_state"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Geen refresh token ontvangen")
}

func TestHandleGoogleCallback_GetUserInfoFails(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=wrong_access_token_code&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "test_state"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Kon gebruikersinfo niet ophalen")
	mockStore.AssertNotCalled(t, "CreateUser")
}

func TestHandleGoogleCallback_CreateUserFails(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	t.Setenv("JWT_SECRET_KEY", "een-zeer-geheim-geheim-voor-jwt")

	mockStore.On("CreateUser", mock.Anything, "test@example.com", "Test User").
		Return(domain.User{}, fmt.Errorf("database connectie faalt")).
		Once()

	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=valid_code&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "test_state"})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Kon gebruiker niet aanmaken")
	mockStore.AssertExpectations(t)
}

func TestHandleGoogleCallback_UpsertAccountFails(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	t.Setenv("JWT_SECRET_KEY", "een-zeer-geheim-geheim-voor-jwt")

	testUserID := uuid.New()

	mockStore.On("CreateUser", mock.Anything, "test@example.com", "Test User").
		Return(domain.User{BaseEntity: domain.BaseEntity{ID: testUserID}, Email: "test@example.com", Name: stringPtr("Test User")}, nil).
		Once()

	// --- HIER IS DE FIX ---
	// mock.AnythingOfType("store.UpsertConnectedAccountParams") -> mock.Anything
	// De library raakt in de war. mock.Anything is de veiligste gok.
	mockStore.On("UpsertConnectedAccount", mock.Anything, mock.Anything).
		Return(domain.ConnectedAccount{}, fmt.Errorf("unieke constraint geschonden")).
		Once()
	// --- EINDE FIX ---

	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=valid_code&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "test_state"})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Kon account niet koppelen")
	mockStore.AssertExpectations(t)
}

func TestHandleGoogleCallback_JWTFails(t *testing.T) {
	_, mockStore, testLogger, oauthConfig := setupCallbackTest(t)
	t.Setenv("CLIENT_BASE_URL", "http://mijnfrontend.com")

	testUserID := uuid.New()
	testAccountID := uuid.New()

	mockStore.On("CreateUser", mock.Anything, "test@example.com", "Test User").
		Return(domain.User{BaseEntity: domain.BaseEntity{ID: testUserID}, Email: "test@example.com", Name: stringPtr("Test User")}, nil).
		Once()

	// --- HIER IS DE FIX ---
	// mock.AnythingOfType("store.UpsertConnectedAccountParams") -> mock.Anything
	mockStore.On("UpsertConnectedAccount", mock.Anything, mock.Anything).
		Return(domain.ConnectedAccount{ID: testAccountID, UserID: testUserID, Email: "test@example.com"}, nil).
		Once()
	// --- EINDE FIX ---

	handler := HandleGoogleCallback(mockStore, oauthConfig, testLogger)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=valid_code&state=test_state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "test_state"})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Kon authenticatie-token niet genereren")
	mockStore.AssertExpectations(t)
}

// stringPtr is een helper om een *string from a string te maken.
func stringPtr(s string) *string {
	return &s
}
