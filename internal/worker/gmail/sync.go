// Package gmail handles Gmail-related background tasks.
package gmail

import (
	"agenda-automator-api/internal/domain"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/gmail/v1"
)

// fetchRecentMessages fetches recent messages for full sync
func (gp *GmailProcessor) fetchRecentMessages(srv *gmail.Service, _ domain.ConnectedAccount) ([]*gmail.Message, error) {
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	query := fmt.Sprintf("after:%d", oneDayAgo.Unix())

	listCall := srv.Users.Messages.List("me").Q(query).MaxResults(100)
	messages, err := listCall.Do()
	if err != nil {
		return nil, fmt.Errorf("could not list messages: %w", err)
	}

	var messageDetails []*gmail.Message
	for _, msg := range messages.Messages {
		fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Format("metadata").Do()
		if err != nil {
			log.Printf("[Gmail] Could not fetch message %s: %v", msg.Id, err)
			continue
		}
		messageDetails = append(messageDetails, fullMsg)
	}

	return messageDetails, nil
}

// processHistoryItems extracts messages from history items.
func (gp *GmailProcessor) processHistoryItems(
	history *gmail.ListHistoryResponse,
	srv *gmail.Service,
) []*gmail.Message {
	var messages []*gmail.Message

	for _, historyItem := range history.History {
		for _, msg := range historyItem.Messages {
			fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Format("metadata").Do()
			if err != nil {
				log.Printf("[Gmail] Could not fetch message %s from history: %v", msg.Id, err)
				continue
			}
			messages = append(messages, fullMsg)
		}
	}

	return messages
}
