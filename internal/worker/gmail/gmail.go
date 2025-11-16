package gmail

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailProcessor handles Gmail message processing
type GmailProcessor struct {
	store store.Storer
}

// NewGmailProcessor creates a new Gmail processor
func NewGmailProcessor(s store.Storer) *GmailProcessor {
	return &GmailProcessor{
		store: s,
	}
}

// ProcessMessages processes Gmail messages for automation rules
func (gp *GmailProcessor) ProcessMessages(ctx context.Context, acc domain.ConnectedAccount, token *oauth2.Token) error {
	// Create Gmail service
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("could not create Gmail service: %w", err)
	}

	// Get current sync state
	historyID, lastSync, err := gp.store.GetGmailSyncState(ctx, acc.ID)
	if err != nil {
		log.Printf("[Gmail] Could not get Gmail sync state for %s: %v", acc.Email, err)
	}

	// Fetch Gmail rules
	rules, err := gp.store.GetGmailRulesForAccount(ctx, acc.ID)
	if err != nil {
		return fmt.Errorf("could not fetch Gmail rules: %w", err)
	}

	activeRules := make([]domain.GmailAutomationRule, 0)
	for _, rule := range rules {
		if rule.IsActive {
			activeRules = append(activeRules, rule)
		}
	}

	if len(activeRules) == 0 {
		log.Printf("[Gmail] No active Gmail rules for %s", acc.Email)
		return nil
	}

	log.Printf("[Gmail] Processing Gmail for %s with %d active rules", acc.Email, len(activeRules))

	// Use History API for incremental sync if possible
	var messagesToProcess []*gmail.Message
	if historyID != nil && lastSync != nil {
		historyIDUint, err := strconv.ParseUint(*historyID, 10, 64)
		if err != nil {
			log.Printf("[Gmail] Invalid history ID format for %s, falling back to full sync: %v", acc.Email, err)
			messagesToProcess, err = gp.fetchRecentMessages(srv, acc)
			if err != nil {
				return fmt.Errorf("could not fetch recent messages: %w", err)
			}
		} else {
			historyCall := srv.Users.History.List("me").StartHistoryId(historyIDUint)
			history, err := historyCall.Do()
			if err != nil {
				log.Printf("[Gmail] History API failed for %s, falling back to full sync: %v", acc.Email, err)
				messagesToProcess, err = gp.fetchRecentMessages(srv, acc)
				if err != nil {
					return fmt.Errorf("could not fetch recent messages: %w", err)
				}
			} else {
				messagesToProcess, err = gp.processHistoryItems(history, srv)
				if err != nil {
					return fmt.Errorf("could not process history items: %w", err)
				}

				if history.HistoryId != 0 {
					newHistoryID := fmt.Sprintf("%d", history.HistoryId)
					err = gp.store.UpdateGmailSyncState(ctx, acc.ID, newHistoryID, time.Now())
					if err != nil {
						log.Printf("[Gmail] Failed to update Gmail sync state: %v", err)
					}
				}
			}
		}
	} else {
		// Full sync
		messagesToProcess, err = gp.fetchRecentMessages(srv, acc)
		if err != nil {
			return fmt.Errorf("could not fetch recent messages: %w", err)
		}

		profile, err := srv.Users.GetProfile("me").Do()
		if err == nil && profile.HistoryId != 0 {
			initialHistoryID := fmt.Sprintf("%d", profile.HistoryId)
			err = gp.store.UpdateGmailSyncState(ctx, acc.ID, initialHistoryID, time.Now())
			if err != nil {
				log.Printf("[Gmail] Failed to store initial Gmail sync state: %v", err)
			}
		}
	}

	// Process messages
	for _, message := range messagesToProcess {
		err = gp.processMessageAgainstRules(ctx, srv, acc, message, activeRules)
		if err != nil {
			log.Printf("[Gmail] Error processing message %s: %v", message.Id, err)
		}
	}

	log.Printf("[Gmail] Completed Gmail processing for %s", acc.Email)
	return nil
}

