package common

import (
	"agenda-automator-api/internal/store" // logger package was hier niet nodig
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// contextKey for user ID
type contextKey string

var UserContextKey contextKey = "user_id"

// GetUserIDFromContext haalt de user ID op die door de middleware in de context is gezet
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(UserContextKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("missing or invalid user ID in context")
	}
	return userID, nil
}

// WriteJSON schrijft een standaard JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}, logger *zap.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error(
			"failed to write JSON response",
			zap.Error(err),
			zap.Int("status", status),
			zap.String("component", "api"),
		)
	}
}

// WriteJSONError schrijft een standaard JSON error response
func WriteJSONError(w http.ResponseWriter, status int, message string, logger *zap.Logger) {
	WriteJSON(w, status, map[string]string{"error": message}, logger)
}

// GetCalendarClient initialiseert een Google Calendar client met token refresh.
func GetCalendarClient(
	ctx context.Context,
	store store.Storer,
	accountID uuid.UUID,
	logger *zap.Logger,
) (*calendar.Service, error) {
	// BELANGRIJK: Gebruik context.Background() voor externe calls,
	// NIET de 'ctx' van de request, om header-vervuiling te voorkomen.
	cleanCtx := context.Background()

	// 1. Haal het token op (deze functie gebruikt de DB context 'ctx', maar 'cleanCtx' voor de refresh)
	// GECORRIGEERD: 'E' verwijderd
	token, err := store.GetValidTokenForAccount(ctx, accountID)
	if err != nil {
		logger.Error(
			"failed to get valid token for calendar client",
			zap.Error(err),
			zap.String("account_id", accountID.String()),
			zap.String("component", "api"),
		)
		return nil, fmt.Errorf("kon geen geldig token voor account ophalen: %w", err)
	}

	// 2. Maak de client en service aan met de schone context
	client := oauth2.NewClient(cleanCtx, oauth2.StaticTokenSource(token))

	return calendar.NewService(cleanCtx, option.WithHTTPClient(client))
}

// GetGmailClient initialiseert een Google Gmail client met token refresh.
func GetGmailClient(
	ctx context.Context,
	store store.Storer,
	accountID uuid.UUID,
	logger *zap.Logger,
) (*gmail.Service, error) {
	// BELANGRIJK: Gebruik context.Background() voor externe calls,
	// NIET de 'ctx' van de request, om header-vervuiling te voorkomen.
	cleanCtx := context.Background()

	// 1. Haal het token op (deze functie gebruikt de DB context 'ctx', maar 'cleanCtx' voor de refresh)
	token, err := store.GetValidTokenForAccount(ctx, accountID)
	if err != nil {
		logger.Error(
			"failed to get valid token for Gmail client",
			zap.Error(err),
			zap.String("account_id", accountID.String()),
			zap.String("component", "api"),
		)
		return nil, fmt.Errorf("kon geen geldig token voor account ophalen: %w", err)
	}

	// 2. Maak de client en service aan met de schone context
	client := oauth2.NewClient(cleanCtx, oauth2.StaticTokenSource(token))

	return gmail.NewService(cleanCtx, option.WithHTTPClient(client))
}

// ParseEmailAddresses parses a comma-separated string of email addresses.
func ParseEmailAddresses(emailString string) []string {
	if emailString == "" {
		return []string{}
	}

	// Simple parsing - split by comma and trim spaces
	// In a production app, you'd want more robust email parsing
	parts := strings.Split(emailString, ",")
	var emails []string
	for _, part := range parts {
		email := strings.TrimSpace(part)
		if email != "" {
			emails = append(emails, email)
		}
	}
	return emails
}

// ExtractMessageBody extracts the message body from a Gmail message payload.
func ExtractMessageBody(payload *gmail.MessagePart) string {
	if payload == nil {
		return ""
	}

	// If it's a simple message (no parts), return the body directly
	if payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(data)
		}
	}

	// If it has parts, look for text/plain or text/html
	if payload.Parts != nil {
		for _, part := range payload.Parts {
			// Skip attachments
			if part.Filename != "" {
				continue
			}

			// Check for text/plain first
			if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err == nil {
					return string(data)
				}
			}

			// If no plain text, check for text/html
			if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err == nil {
					// For HTML, we could strip tags, but for now return as-is
					return string(data)
				}
			}

			// Recursively check nested parts
			if nestedBody := ExtractMessageBody(part); nestedBody != "" {
				return nestedBody
			}
		}
	}

	return ""
}
