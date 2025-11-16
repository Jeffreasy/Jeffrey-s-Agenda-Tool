package gmail

import (
	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"encoding/base64"
	"encoding/json"
	"fmt"

	// "log" // <-- VERWIJDERD
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap" // <-- TOEGEVOEGD
	"google.golang.org/api/gmail/v1"
)

// HandleGetGmailMessages retrieves Gmail messages.
func HandleGetGmailMessages(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log) // <-- log meegegeven
			return
		}

		// Query parameters
		query := r.URL.Query().Get("q") // Gmail search query
		labelIds := r.URL.Query()["labelIds"]
		maxResultsStr := r.URL.Query().Get("maxResults")
		maxResults := 50 // default
		if maxResultsStr != "" {
			if parsed, err := strconv.Atoi(maxResultsStr); err == nil && parsed > 0 && parsed <= 500 {
				maxResults = parsed
			}
		}

		ctx := r.Context()
		// AANGEPAST: log meegegeven (vereist aanpassing in common.GetGmailClient)
		client, err := common.GetGmailClient(ctx, store, accountID, log)
		if err != nil {
			log.Error("HANDLER ERROR [getGmailClient]", zap.Error(err)) // <-- AANGEPAST
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail client niet initialiseren", log)
			return
		}

		listCall := client.Users.Messages.List("me").MaxResults(int64(maxResults))
		if query != "" {
			listCall = listCall.Q(query)
		}
		if len(labelIds) > 0 {
			listCall = listCall.LabelIds(labelIds...)
		}

		messages, err := listCall.Do()
		if err != nil {
			log.Error("HANDLER ERROR [client.Users.Messages.List]", zap.Error(err)) // <-- AANGEPAST
			common.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon berichten niet ophalen: %v", err), log)
			return
		}

		// Transform Gmail API response to our expected format
		// Try to fetch full message details, fall back to metadata if scope doesn't allow
		var transformedMessages []map[string]interface{}
		for _, msg := range messages.Messages {
			var fullMsg *gmail.Message
			var err error

			// Try to get full message first
			fullMsg, err = client.Users.Messages.Get("me", msg.Id).Format("full").Do()
			if err != nil {
				// If full access fails (403), try metadata only
				// This is expected when token only has gmail.metadata scope
				fullMsg, err = client.Users.Messages.Get("me", msg.Id).Format("metadata").Do()
				if err != nil {
					log.Warn("Error fetching message metadata", zap.String("msg_id", msg.Id), zap.Error(err)) // <-- AANGEPAST
					continue
				}
			}

			// Extract headers
			var subject, sender, to, cc, bcc string
			var receivedAt time.Time
			hasAttachments := false
			isStarred := false

			for _, header := range fullMsg.Payload.Headers {
				switch header.Name {
				case "Subject":
					subject = header.Value
				case "From":
					sender = header.Value
				case "To":
					to = header.Value
				case "Cc":
					cc = header.Value
				case "Bcc":
					bcc = header.Value
				case "Date":
					if parsed, err := time.Parse(time.RFC1123Z, header.Value); err == nil {
						receivedAt = parsed
					} else if parsed, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", header.Value); err == nil {
						receivedAt = parsed
					}
				}
			}

			// Check for attachments
			if fullMsg.Payload.Parts != nil {
				for _, part := range fullMsg.Payload.Parts {
					if part.Filename != "" {
						hasAttachments = true
						break
					}
				}
			}

			// Check if starred
			for _, labelID := range msg.LabelIds {
				if labelID == "STARRED" {
					isStarred = true
					break
				}
			}

			// Determine status
			status := "read"
			for _, labelID := range msg.LabelIds {
				if labelID == "UNREAD" {
					status = "unread"
					break
				}
			}

			// Extract message body (only available with full access)
			var messageBody string
			if fullMsg.Payload != nil && fullMsg.Payload.Body != nil {
				// We have full access, extract the body
				messageBody = common.ExtractMessageBody(fullMsg.Payload)
			}
			// If we only have metadata access, messageBody remains empty

			// Parse recipients
			var recipients, ccRecipients, bccRecipients []string
			if to != "" {
				recipients = common.ParseEmailAddresses(to)
			}
			if cc != "" {
				ccRecipients = common.ParseEmailAddresses(cc)
			}
			if bcc != "" {
				bccRecipients = common.ParseEmailAddresses(bcc)
			}

			// Use snippet from metadata response if available, otherwise from list response
			snippet := msg.Snippet
			if fullMsg.Snippet != "" {
				snippet = fullMsg.Snippet
			}

			transformedMessages = append(transformedMessages, map[string]interface{}{
				"id":               msg.Id,
				"gmail_message_id": msg.Id,
				"gmail_thread_id":  msg.ThreadId,
				"subject":          subject,
				"sender":           sender,
				"recipients":       recipients,
				"cc_recipients":    ccRecipients,
				"bcc_recipients":   bccRecipients,
				"snippet":          snippet,
				"body":             messageBody,
				"status":           status,
				"is_starred":       isStarred,
				"has_attachments":  hasAttachments,
				"attachment_count": 0, // Would need deeper parsing for accurate count
				"size_estimate":    fullMsg.SizeEstimate,
				"received_at":      receivedAt.Format(time.RFC3339),
				"labels":           msg.LabelIds,
				"last_synced":      time.Now().Format(time.RFC3339),
				"created_at":       time.Now().Format(time.RFC3339),
				"updated_at":       time.Now().Format(time.RFC3339),
			})
		}

		common.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"messages": transformedMessages,
		}, log) // <-- log meegegeven
	}
}

// HandleSendGmailMessage sends an email using Gmail.
func HandleSendGmailMessage(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		var req struct {
			To      []string `json:"to"`
			Cc      []string `json:"cc,omitempty"`
			Bcc     []string `json:"bcc,omitempty"`
			Subject string   `json:"subject"`
			Body    string   `json:"body"`
			IsHTML  bool     `json:"isHtml,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", log)
			return
		}

		ctx := r.Context()
		// AANGEPAST: log meegegeven
		client, err := common.GetGmailClient(ctx, store, accountID, log)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail client niet initialiseren", log)
			return
		}

		// Create the email message
		var message gmail.Message

		// For simplicity, we'll create a basic text message
		// In a full implementation, you'd want to properly encode MIME messages
		contentType := "text/plain"
		if req.IsHTML {
			contentType = "text/html"
		}

		// Build recipients string
		toRecipients := strings.Join(req.To, ",")
		var ccRecipients, bccRecipients string
		if len(req.Cc) > 0 {
			ccRecipients = "Cc: " + strings.Join(req.Cc, ",") + "\r\n"
		}
		if len(req.Bcc) > 0 {
			bccRecipients = "Bcc: " + strings.Join(req.Bcc, ",") + "\r\n"
		}

		// Create raw email content
		rawMessage := fmt.Sprintf("To: %s\r\n%s%sSubject: %s\r\nContent-Type: %s; charset=UTF-8\r\n\r\n%s",
			toRecipients, ccRecipients, bccRecipients, req.Subject, contentType, req.Body)

		message.Raw = base64.URLEncoding.EncodeToString([]byte(rawMessage))

		sentMessage, err := client.Users.Messages.Send("me", &message).Do()
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon email niet versturen: %v", err), log)
			return
		}

		common.WriteJSON(w, http.StatusOK, sentMessage, log)
	}
}

// HandleGetGmailLabels retrieves Gmail labels.
func HandleGetGmailLabels(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		ctx := r.Context()
		// AANGEPAST: log meegegeven
		client, err := common.GetGmailClient(ctx, store, accountID, log)
		if err != nil {
			log.Error("HANDLER ERROR [getGmailClient]", zap.Error(err)) // <-- AANGEPAST
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail client niet initialiseren", log)
			return
		}

		labels, err := client.Users.Labels.List("me").Do()
		if err != nil {
			log.Error("HANDLER ERROR [client.Users.Labels.List]", zap.Error(err)) // <-- AANGEPAST
			common.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon labels niet ophalen: %v", err), log)
			return
		}

		common.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"labels": labels.Labels,
		}, log)
	}
}

// HandleCreateGmailDraft creates a Gmail draft.
func HandleCreateGmailDraft(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		var req struct {
			To      []string `json:"to"`
			Cc      []string `json:"cc,omitempty"`
			Bcc     []string `json:"bcc,omitempty"`
			Subject string   `json:"subject"`
			Body    string   `json:"body"`
			IsHTML  bool     `json:"isHtml,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", log)
			return
		}

		ctx := r.Context()
		// AANGEPAST: log meegegeven
		client, err := common.GetGmailClient(ctx, store, accountID, log)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail client niet initialiseren", log)
			return
		}

		// Create draft message similar to send message
		var draft gmail.Draft
		var message gmail.Message

		contentType := "text/plain"
		if req.IsHTML {
			contentType = "text/html"
		}

		toRecipients := strings.Join(req.To, ",")
		var ccRecipients, bccRecipients string
		if len(req.Cc) > 0 {
			ccRecipients = "Cc: " + strings.Join(req.Cc, ",") + "\r\n"
		}
		if len(req.Bcc) > 0 {
			bccRecipients = "Bcc: " + strings.Join(req.Bcc, ",") + "\r\n"
		}

		rawMessage := fmt.Sprintf("To: %s\r\n%s%sSubject: %s\r\nContent-Type: %s; charset=UTF-8\r\n\r\n%s",
			toRecipients, ccRecipients, bccRecipients, req.Subject, contentType, req.Body)

		message.Raw = base64.URLEncoding.EncodeToString([]byte(rawMessage))
		draft.Message = &message

		createdDraft, err := client.Users.Drafts.Create("me", &draft).Do()
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon draft niet creren: %v", err), log)
			return
		}

		common.WriteJSON(w, http.StatusCreated, createdDraft, log)
	}
}

// HandleGetGmailDrafts retrieves Gmail drafts.
func HandleGetGmailDrafts(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		ctx := r.Context()
		// AANGEPAST: log meegegeven
		client, err := common.GetGmailClient(ctx, store, accountID, log)
		if err != nil {
			log.Error("HANDLER ERROR [getGmailClient]", zap.Error(err)) // <-- AANGEPAST
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail client niet initialiseren", log)
			return
		}

		drafts, err := client.Users.Drafts.List("me").Do()
		if err != nil {
			log.Error("HANDLER ERROR [client.Users.Drafts.List]", zap.Error(err)) // <-- AANGEPAST
			common.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Kon drafts niet ophalen: %v", err), log)
			return
		}

		common.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"drafts": drafts.Drafts,
		}, log)
	}
}

// HandleCreateGmailRule creert een nieuwe Gmail automation rule
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleCreateGmailRule(storer store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log)
			return
		}

		account, err := storer.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusNotFound, "Account niet gevonden", log)
			return
		}

		var req domain.GmailAutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", log)
			return
		}

		params := store.CreateGmailAutomationRuleParams{
			ConnectedAccountID: accountID,
			Name:               req.Name,
			Description:        req.Description,
			IsActive:           req.IsActive,
			TriggerType:        req.TriggerType,
			TriggerConditions:  req.TriggerConditions,
			ActionType:         req.ActionType,
			ActionParams:       req.ActionParams,
			Priority:           req.Priority,
		}

		rule, err := storer.CreateGmailAutomationRule(r.Context(), params)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail rule niet creren", log)
			return
		}

		common.WriteJSON(w, http.StatusCreated, rule, log)
	}
}

// HandleGetGmailRules haalt Gmail automation rules op
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleGetGmailRules(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log)
			return
		}

		account, err := store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusNotFound, "Account niet gevonden", log)
			return
		}

		rules, err := store.GetGmailRulesForAccount(r.Context(), accountID)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon Gmail rules niet ophalen", log)
			return
		}

		common.WriteJSON(w, http.StatusOK, rules, log)
	}
}
