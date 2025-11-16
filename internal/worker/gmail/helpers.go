// Package gmail handles Gmail-related background tasks.
package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"

	"github.com/google/uuid"
	"google.golang.org/api/gmail/v1"
)

// storeMessageInDB stores a Gmail message in the database.
func (gp *GmailProcessor) storeMessageInDB(
	ctx context.Context,
	acc domain.ConnectedAccount,
	message *gmail.Message,
) error {
	subject := gp.getHeaderValue(message.Payload.Headers, "Subject")
	sender := gp.getHeaderValue(message.Payload.Headers, "From")
	snippet := message.Snippet

	var toRecipients, ccRecipients, bccRecipients []string
	if message.Payload != nil {
		toRecipients = gp.extractRecipients(message.Payload.Headers, "To")
		ccRecipients = gp.extractRecipients(message.Payload.Headers, "Cc")
		bccRecipients = gp.extractRecipients(message.Payload.Headers, "Bcc")
	}

	var status domain.GmailMessageStatus
	if gp.hasLabel(message.LabelIds, "UNREAD") {
		status = domain.GmailUnread
	} else {
		status = domain.GmailRead
	}

	isStarred := gp.hasLabel(message.LabelIds, "STARRED")
	hasAttachments := len(message.Payload.Parts) > 1 ||
		(message.Payload.Body != nil && message.Payload.Body.Size > 0 && len(message.Payload.Parts) > 0)

	receivedAt := time.Unix(message.InternalDate/1000, 0)

	params := store.StoreGmailMessageParams{
		ConnectedAccountID: acc.ID,
		GmailMessageID:     message.Id,
		GmailThreadID:      message.ThreadId,
		Subject:            subject,
		Sender:             sender,
		Recipients:         toRecipients,
		CcRecipients:       ccRecipients,
		BccRecipients:      bccRecipients,
		Snippet:            &snippet,
		Status:             status,
		IsStarred:          isStarred,
		HasAttachments:     hasAttachments,
		AttachmentCount:    gp.countAttachments(message.Payload),
		SizeEstimate:       &message.SizeEstimate,
		ReceivedAt:         receivedAt,
		Labels:             message.LabelIds,
	}

	return gp.store.StoreGmailMessage(ctx, params)
}

// Helper functions
func (gp *GmailProcessor) getHeaderValue(headers []*gmail.MessagePartHeader, name string) *string {
	for _, header := range headers {
		if header.Name == name {
			return &header.Value
		}
	}
	return nil
}

func (gp *GmailProcessor) extractRecipients(headers []*gmail.MessagePartHeader, headerName string) []string {
	headerValue := gp.getHeaderValue(headers, headerName)
	if headerValue == nil {
		return []string{}
	}

	recipients := strings.Split(*headerValue, ",")
	for i, recipient := range recipients {
		recipients[i] = strings.TrimSpace(recipient)
	}
	return recipients
}

func (gp *GmailProcessor) hasLabel(labelIds []string, label string) bool {
	for _, l := range labelIds {
		if l == label {
			return true
		}
	}
	return false
}

func (gp *GmailProcessor) countAttachments(payload *gmail.MessagePart) int {
	if payload == nil {
		return 0
	}

	count := 0
	if payload.Filename != "" {
		count++
	}

	for _, part := range payload.Parts {
		count += gp.countAttachments(part)
	}

	return count
}

// Gmail label helpers
func (gp *GmailProcessor) getOrCreateLabel(srv *gmail.Service, name string) (*gmail.Label, error) {
	labels, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, err
	}

	for _, label := range labels.Labels {
		if label.Name == name {
			return label, nil
		}
	}

	newLabel := &gmail.Label{
		Name:                  name,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}
	return srv.Users.Labels.Create("me", newLabel).Do()
}

func (gp *GmailProcessor) getLabelByName(srv *gmail.Service, name string) (*gmail.Label, error) {
	labels, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, err
	}

	for _, label := range labels.Labels {
		if label.Name == name {
			return label, nil
		}
	}

	return nil, fmt.Errorf("label not found: %s", name)
}

func (gp *GmailProcessor) createReplyRaw(originalMessage *gmail.Message, replyText, fromEmail string) string {
	subject := gp.getHeaderValue(originalMessage.Payload.Headers, "Subject")
	if subject == nil {
		subject = stringPtr("Re:")
	} else if !strings.HasPrefix(*subject, "Re:") {
		subject = stringPtr("Re: " + *subject)
	}

	to := gp.getHeaderValue(originalMessage.Payload.Headers, "From")
	messageID := gp.getHeaderValue(originalMessage.Payload.Headers, "Message-ID")

	rawMessage := fmt.Sprintf(
		"To: %s\r\nFrom: %s\r\nSubject: %s\r\n",
		*to, fromEmail, *subject,
	)

	if messageID != nil {
		rawMessage += fmt.Sprintf("References: %s\r\nIn-Reply-To: %s\r\n", *messageID, *messageID)
	}

	rawMessage += "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + replyText

	return base64.URLEncoding.EncodeToString([]byte(rawMessage))
}

// Logging helpers
func (gp *GmailProcessor) logGmailAutomationSuccess(
	ctx context.Context,
	accountID uuid.UUID,
	ruleID *uuid.UUID,
	messageID, threadID, details string,
) {
	params := store.CreateLogParams{
		ConnectedAccountID: accountID,
		RuleID:             *ruleID,
		Status:             domain.LogSuccess,
		TriggerDetails: json.RawMessage(
			fmt.Sprintf(
				`{"gmail_message_id": "%s", "gmail_thread_id": "%s"}`,
				messageID,
				threadID,
			),
		),
		ActionDetails: json.RawMessage(fmt.Sprintf(`{"details": "%s"}`, details)),
	}
	gp.store.CreateAutomationLog(ctx, params)
}

func (gp *GmailProcessor) logGmailAutomationFailure(
	ctx context.Context,
	accountID uuid.UUID,
	ruleID *uuid.UUID,
	messageID, threadID, errorMsg string,
) {
	params := store.CreateLogParams{
		ConnectedAccountID: accountID,
		RuleID:             *ruleID,
		Status:             domain.LogFailure,
		TriggerDetails: json.RawMessage(
			fmt.Sprintf(
				`{"gmail_message_id": "%s", "gmail_thread_id": "%s"}`,
				messageID,
				threadID,
			),
		),
		ErrorMessage: errorMsg,
	}
	gp.store.CreateAutomationLog(ctx, params)
}

func stringPtr(s string) *string {
	return &s
}
