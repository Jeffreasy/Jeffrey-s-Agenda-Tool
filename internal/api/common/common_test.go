package common

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/api/gmail/v1"
)

func TestGetUserIDFromContext(t *testing.T) {
	t.Run("Valid user ID in context", func(t *testing.T) {
		userID := uuid.New()
		ctx := context.WithValue(context.Background(), UserContextKey, userID)

		result, err := GetUserIDFromContext(ctx)

		assert.NoError(t, err)
		assert.Equal(t, userID, result)
	})

	t.Run("No user ID in context", func(t *testing.T) {
		ctx := context.Background()

		result, err := GetUserIDFromContext(ctx)

		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, result)
		assert.Contains(t, err.Error(), "missing or invalid user ID in context")
	})

	t.Run("Invalid type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserContextKey, "not-a-uuid")

		result, err := GetUserIDFromContext(ctx)

		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, result)
		assert.Contains(t, err.Error(), "missing or invalid user ID in context")
	})
}

func TestWriteJSON(t *testing.T) {
	logger := zap.NewNop()

	t.Run("Successful JSON write", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "test"}

		WriteJSON(w, http.StatusOK, data, logger)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		expected := `{"message":"test"}`
		assert.JSONEq(t, expected, w.Body.String())
	})

	t.Run("JSON write with different status", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]int{"count": 42}

		WriteJSON(w, http.StatusCreated, data, logger)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		expected := `{"count":42}`
		assert.JSONEq(t, expected, w.Body.String())
	})
}

func TestWriteJSONError(t *testing.T) {
	logger := zap.NewNop()

	t.Run("Error response", func(t *testing.T) {
		w := httptest.NewRecorder()
		message := "Something went wrong"

		WriteJSONError(w, http.StatusBadRequest, message, logger)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		expected := `{"error":"Something went wrong"}`
		assert.JSONEq(t, expected, w.Body.String())
	})
}

func TestParseEmailAddresses(t *testing.T) {
	t.Run("Single email", func(t *testing.T) {
		result := ParseEmailAddresses("test@example.com")
		expected := []string{"test@example.com"}
		assert.Equal(t, expected, result)
	})

	t.Run("Multiple emails", func(t *testing.T) {
		result := ParseEmailAddresses("test1@example.com,test2@example.com,test3@example.com")
		expected := []string{"test1@example.com", "test2@example.com", "test3@example.com"}
		assert.Equal(t, expected, result)
	})

	t.Run("Emails with spaces", func(t *testing.T) {
		result := ParseEmailAddresses(" test1@example.com , test2@example.com ")
		expected := []string{"test1@example.com", "test2@example.com"}
		assert.Equal(t, expected, result)
	})

	t.Run("Empty string", func(t *testing.T) {
		result := ParseEmailAddresses("")
		expected := []string{}
		assert.Equal(t, expected, result)
	})

	t.Run("Empty parts", func(t *testing.T) {
		result := ParseEmailAddresses("test@example.com,,test2@example.com,")
		expected := []string{"test@example.com", "test2@example.com"}
		assert.Equal(t, expected, result)
	})
}

func TestExtractMessageBody(t *testing.T) {
	t.Run("Nil payload", func(t *testing.T) {
		result := ExtractMessageBody(nil)
		assert.Equal(t, "", result)
	})

	t.Run("Simple message with body", func(t *testing.T) {
		bodyData := base64.URLEncoding.EncodeToString([]byte("Hello world"))
		payload := &gmail.MessagePart{
			Body: &gmail.MessagePartBody{
				Data: bodyData,
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "Hello world", result)
	})

	t.Run("Message with text/plain part", func(t *testing.T) {
		plainData := base64.URLEncoding.EncodeToString([]byte("Plain text content"))
		payload := &gmail.MessagePart{
			Parts: []*gmail.MessagePart{
				{
					MimeType: "text/plain",
					Body: &gmail.MessagePartBody{
						Data: plainData,
					},
				},
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "Plain text content", result)
	})

	t.Run("Message with text/html part (fallback)", func(t *testing.T) {
		htmlData := base64.URLEncoding.EncodeToString([]byte("<p>HTML content</p>"))
		payload := &gmail.MessagePart{
			Parts: []*gmail.MessagePart{
				{
					MimeType: "text/html",
					Body: &gmail.MessagePartBody{
						Data: htmlData,
					},
				},
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "<p>HTML content</p>", result)
	})

	t.Run("Message with both plain and html (prefers plain)", func(t *testing.T) {
		plainData := base64.URLEncoding.EncodeToString([]byte("Plain text"))
		htmlData := base64.URLEncoding.EncodeToString([]byte("<p>HTML text</p>"))
		payload := &gmail.MessagePart{
			Parts: []*gmail.MessagePart{
				{
					MimeType: "text/plain",
					Body: &gmail.MessagePartBody{
						Data: plainData,
					},
				},
				{
					MimeType: "text/html",
					Body: &gmail.MessagePartBody{
						Data: htmlData,
					},
				},
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "Plain text", result)
	})

	t.Run("Message with attachment (skips attachment)", func(t *testing.T) {
		plainData := base64.URLEncoding.EncodeToString([]byte("Actual content"))
		payload := &gmail.MessagePart{
			Parts: []*gmail.MessagePart{
				{
					Filename: "attachment.pdf",
					MimeType: "application/pdf",
				},
				{
					MimeType: "text/plain",
					Body: &gmail.MessagePartBody{
						Data: plainData,
					},
				},
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "Actual content", result)
	})

	t.Run("Nested parts", func(t *testing.T) {
		nestedData := base64.URLEncoding.EncodeToString([]byte("Nested content"))
		payload := &gmail.MessagePart{
			Parts: []*gmail.MessagePart{
				{
					Parts: []*gmail.MessagePart{
						{
							MimeType: "text/plain",
							Body: &gmail.MessagePartBody{
								Data: nestedData,
							},
						},
					},
				},
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "Nested content", result)
	})

	t.Run("Invalid base64 data", func(t *testing.T) {
		payload := &gmail.MessagePart{
			Body: &gmail.MessagePartBody{
				Data: "invalid-base64!",
			},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "", result)
	})

	t.Run("Empty parts", func(t *testing.T) {
		payload := &gmail.MessagePart{
			Parts: []*gmail.MessagePart{},
		}

		result := ExtractMessageBody(payload)
		assert.Equal(t, "", result)
	})
}
