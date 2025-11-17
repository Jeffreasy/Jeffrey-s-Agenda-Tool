package gmail

import (
	"agenda-automator-api/internal/domain"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/gmail/v1"
)

// Helper om een processor te maken (geen mocks nodig voor deze tests)
func newTestProcessor() *GmailProcessor {
	return &GmailProcessor{store: nil} // Store is niet nodig voor checkRuleMatch
}

// Test 1: Sender Match - Wel match
func TestGmail_checkRuleMatch_SenderMatch(t *testing.T) {
	gp := newTestProcessor()

	// Arrange
	msg := &gmail.Message{
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: "test@example.com"},
			},
		},
	}

	cond, _ := json.Marshal(map[string]string{"sender_pattern": "example.com"})
	rule := domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{ // FIX: Velden in BaseAutomationRule geplaatst
			TriggerConditions: cond,
		},
		TriggerType: domain.GmailTriggerSenderMatch,
	}

	// Act
	matches, err := gp.checkRuleMatch(msg, rule)

	// Assert
	assert.NoError(t, err)
	assert.True(t, matches)
}

// Test 2: Sender Match - Geen match
func TestGmail_checkRuleMatch_SenderNoMatch(t *testing.T) {
	gp := newTestProcessor()

	// Arrange
	msg := &gmail.Message{
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: "test@google.com"},
			},
		},
	}

	cond, _ := json.Marshal(map[string]string{"sender_pattern": "example.com"})
	rule := domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{ // FIX: Velden in BaseAutomationRule geplaatst
			TriggerConditions: cond,
		},
		TriggerType: domain.GmailTriggerSenderMatch,
	}

	// Act
	matches, err := gp.checkRuleMatch(msg, rule)

	// Assert
	assert.NoError(t, err)
	assert.False(t, matches)
}

// Test 3: Subject Match - Wel match (case-insensitive)
func TestGmail_checkRuleMatch_SubjectMatch(t *testing.T) {
	gp := newTestProcessor()

	// Arrange
	msg := &gmail.Message{
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "Subject", Value: "URGENT: Please reply"},
			},
		},
	}

	cond, _ := json.Marshal(map[string]string{"subject_pattern": "urgent"}) // lower case
	rule := domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{ // FIX: Velden in BaseAutomationRule geplaatst
			TriggerConditions: cond,
		},
		TriggerType: domain.GmailTriggerSubjectMatch,
	}

	// Act
	matches, err := gp.checkRuleMatch(msg, rule)

	// Assert
	assert.NoError(t, err)
	assert.True(t, matches)
}

// Test 4: Starred Match - Wel match
func TestGmail_checkRuleMatch_Starred(t *testing.T) {
	gp := newTestProcessor()

	// Arrange
	msg := &gmail.Message{
		LabelIds: []string{"INBOX", "STARRED", "IMPORTANT"},
	}
	rule := domain.GmailAutomationRule{
		TriggerType: domain.GmailTriggerStarred,
	}

	// Act
	matches, err := gp.checkRuleMatch(msg, rule)

	// Assert
	assert.NoError(t, err)
	assert.True(t, matches)
}
