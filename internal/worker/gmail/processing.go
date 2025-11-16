// Package gmail handles Gmail-related background tasks.
package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"agenda-automator-api/internal/domain"

	"google.golang.org/api/gmail/v1"
)

// processMessageAgainstRules applies automation rules to a message.
func (gp *GmailProcessor) processMessageAgainstRules(
	ctx context.Context,
	srv *gmail.Service,
	acc *domain.ConnectedAccount,
	message *gmail.Message,
	rules []domain.GmailAutomationRule,
) error {
	// Store message in database first
	err := gp.storeMessageInDB(ctx, acc, message)
	if err != nil {
		return fmt.Errorf("could not store message: %w", err)
	}

	// Apply each active rule
	for _, rule := range rules {
		matches, err := gp.checkRuleMatch(message, rule)
		if err != nil {
			log.Printf("[Gmail] Error checking rule match for rule %s: %v", rule.ID, err)
			continue
		}

		if matches {
			log.Printf("[Gmail] Message %s matches rule '%s'", message.Id, rule.Name)
			err = gp.executeRuleAction(ctx, srv, acc, message, rule)
			if err != nil {
				log.Printf("[Gmail] Error executing rule action for rule %s: %v", rule.ID, err)
				gp.logGmailAutomationFailure(ctx, acc.ID, &rule.ID, message.Id, message.ThreadId, err.Error())
			} else {
				gp.logGmailAutomationSuccess(ctx, acc.ID, &rule.ID, message.Id, message.ThreadId, "Action executed successfully")
			}
		}
	}

	return nil
}

// checkRuleMatch checks if a message matches a rule's trigger conditions
func (gp *GmailProcessor) checkRuleMatch(message *gmail.Message, rule domain.GmailAutomationRule) (bool, error) {
	switch rule.TriggerType {
	case domain.GmailTriggerNewMessage:
		return true, nil

	case domain.GmailTriggerSenderMatch:
		var conditions struct {
			SenderPattern string `json:"sender_pattern"`
		}
		if err := json.Unmarshal(rule.TriggerConditions, &conditions); err != nil {
			return false, err
		}
		sender := gp.getHeaderValue(message.Payload.Headers, "From")
		if sender != nil && strings.Contains(strings.ToLower(*sender), strings.ToLower(conditions.SenderPattern)) {
			return true, nil
		}

	case domain.GmailTriggerSubjectMatch:
		var conditions struct {
			SubjectPattern string `json:"subject_pattern"`
		}
		if err := json.Unmarshal(rule.TriggerConditions, &conditions); err != nil {
			return false, err
		}
		subject := gp.getHeaderValue(message.Payload.Headers, "Subject")
		if subject != nil && strings.Contains(strings.ToLower(*subject), strings.ToLower(conditions.SubjectPattern)) {
			return true, nil
		}

	case domain.GmailTriggerLabelAdded:
		var conditions struct {
			LabelName string `json:"label_name"`
		}
		if err := json.Unmarshal(rule.TriggerConditions, &conditions); err != nil {
			return false, err
		}
		return gp.hasLabel(message.LabelIds, conditions.LabelName), nil

	case domain.GmailTriggerStarred:
		return gp.hasLabel(message.LabelIds, "STARRED"), nil
	}

	return false, nil
}

// executeRuleAction executes the action defined by a rule.
func (gp *GmailProcessor) executeRuleAction(
	ctx context.Context,
	srv *gmail.Service,
	acc *domain.ConnectedAccount,
	message *gmail.Message,
	rule domain.GmailAutomationRule,
) error {
	switch rule.ActionType {
	case domain.GmailActionAutoReply:
		return gp.executeAutoReply(ctx, srv, acc, message, rule)

	case domain.GmailActionForward:
		return gp.executeForward(ctx, srv, acc, message, rule)

	case domain.GmailActionAddLabel:
		return gp.executeAddLabel(ctx, srv, acc, message, rule)

	case domain.GmailActionRemoveLabel:
		return gp.executeRemoveLabel(ctx, srv, acc, message, rule)

	case domain.GmailActionMarkRead:
		return gp.executeMarkRead(ctx, srv, acc, message, rule)

	case domain.GmailActionMarkUnread:
		return gp.executeMarkUnread(ctx, srv, acc, message, rule)

	case domain.GmailActionArchive:
		return gp.executeArchive(ctx, srv, acc, message, rule)

	case domain.GmailActionTrash:
		return gp.executeTrash(ctx, srv, acc, message, rule)

	case domain.GmailActionStar:
		return gp.executeStar(ctx, srv, acc, message, rule)

	case domain.GmailActionUnstar:
		return gp.executeUnstar(ctx, srv, acc, message, rule)
	}

	return fmt.Errorf("unknown action type: %s", rule.ActionType)
}
