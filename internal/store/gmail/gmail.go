package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"agenda-automator-api/internal/database"
	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateGmailAutomationRuleParams contains parameters for creating Gmail automation rules.
type CreateGmailAutomationRuleParams struct {
	ConnectedAccountID uuid.UUID
	Name               string
	Description        *string
	IsActive           bool
	TriggerType        domain.GmailRuleTriggerType
	TriggerConditions  json.RawMessage
	ActionType         domain.GmailRuleActionType
	ActionParams       json.RawMessage
	Priority           int
}

type UpdateGmailRuleParams struct {
	RuleID            uuid.UUID
	Name              string
	Description       *string
	TriggerType       domain.GmailRuleTriggerType
	TriggerConditions json.RawMessage
	ActionType        domain.GmailRuleActionType
	ActionParams      json.RawMessage
	Priority          int
}

type StoreGmailMessageParams struct {
	ConnectedAccountID uuid.UUID
	GmailMessageID     string
	GmailThreadID      string
	Subject            *string
	Sender             *string
	Recipients         []string
	CcRecipients       []string
	BccRecipients      []string
	Snippet            *string
	Status             domain.GmailMessageStatus
	IsStarred          bool
	HasAttachments     bool
	AttachmentCount    int
	SizeEstimate       *int64
	ReceivedAt         time.Time
	Labels             []string
}

type StoreGmailThreadParams struct {
	ConnectedAccountID uuid.UUID
	GmailThreadID      string
	Subject            *string
	Snippet            *string
	MessageCount       int
	HasUnread          bool
	LastMessageAt      time.Time
	Labels             []string
}

// GmailStore handles Gmail-related database operations
type GmailStore struct {
	db  database.Querier
	log *zap.Logger
}

// NewGmailStore creates a new GmailStore
func NewGmailStore(db database.Querier, log *zap.Logger) *GmailStore {
	return &GmailStore{
		db:  db,
		log: log.With(zap.String("component", "gmail_store")),
	}
}

// CreateGmailAutomationRule creates a new Gmail automation rule.
func (s *GmailStore) CreateGmailAutomationRule(
	ctx context.Context,
	arg CreateGmailAutomationRuleParams,
) (domain.GmailAutomationRule, error) {
	query := `
		INSERT INTO gmail_automation_rules (
			connected_account_id, name, description, is_active, trigger_type,
			trigger_conditions, action_type, action_params, priority
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, connected_account_id, name, description, is_active, trigger_type,
		          trigger_conditions, action_type, action_params, priority, created_at, updated_at;
	`

	row := s.db.QueryRow(ctx, query,
		arg.ConnectedAccountID, arg.Name, arg.Description, arg.IsActive, arg.TriggerType,
		arg.TriggerConditions, arg.ActionType, arg.ActionParams, arg.Priority,
	)

	var rule domain.GmailAutomationRule
	err := row.Scan(
		&rule.ID, &rule.ConnectedAccountID, &rule.Name, &rule.Description, &rule.IsActive,
		&rule.TriggerType, &rule.TriggerConditions, &rule.ActionType, &rule.ActionParams,
		&rule.Priority, &rule.CreatedAt, &rule.UpdatedAt,
	)

	if err != nil {
		return domain.GmailAutomationRule{}, err
	}

	return rule, nil
}

// GetGmailRulesForAccount gets all Gmail automation rules for an account.
func (s *GmailStore) GetGmailRulesForAccount(
	ctx context.Context,
	accountID uuid.UUID,
) ([]domain.GmailAutomationRule, error) {
	query := `
		SELECT id, connected_account_id, name, description, is_active, trigger_type,
		       trigger_conditions, action_type, action_params, priority, created_at, updated_at
		FROM gmail_automation_rules
		WHERE connected_account_id = $1
		ORDER BY priority DESC, created_at DESC;
	`

	rows, err := s.db.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.GmailAutomationRule
	for rows.Next() {
		var rule domain.GmailAutomationRule
		err := rows.Scan(
			&rule.ID, &rule.ConnectedAccountID, &rule.Name, &rule.Description, &rule.IsActive,
			&rule.TriggerType, &rule.TriggerConditions, &rule.ActionType, &rule.ActionParams,
			&rule.Priority, &rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

// UpdateGmailRule updates an existing Gmail automation rule.
func (s *GmailStore) UpdateGmailRule(
	ctx context.Context,
	arg UpdateGmailRuleParams,
) (domain.GmailAutomationRule, error) {
	query := `
		UPDATE gmail_automation_rules
		SET name = $1, description = $2, trigger_type = $3, trigger_conditions = $4,
		    action_type = $5, action_params = $6, priority = $7, updated_at = now()
		WHERE id = $8
		RETURNING id, connected_account_id, name, description, is_active, trigger_type,
		          trigger_conditions, action_type, action_params, priority, created_at, updated_at;
	`

	row := s.db.QueryRow(ctx, query,
		arg.Name, arg.Description, arg.TriggerType, arg.TriggerConditions,
		arg.ActionType, arg.ActionParams, arg.Priority, arg.RuleID,
	)

	var rule domain.GmailAutomationRule
	err := row.Scan(
		&rule.ID, &rule.ConnectedAccountID, &rule.Name, &rule.Description, &rule.IsActive,
		&rule.TriggerType, &rule.TriggerConditions, &rule.ActionType, &rule.ActionParams,
		&rule.Priority, &rule.CreatedAt, &rule.UpdatedAt,
	)

	if err != nil {
		return domain.GmailAutomationRule{}, err
	}

	return rule, nil
}

// DeleteGmailRule deletes a Gmail automation rule
func (s *GmailStore) DeleteGmailRule(ctx context.Context, ruleID uuid.UUID) error {
	s.log.Info("Attempting to delete rule", zap.String("rule_id", ruleID.String()))

	query := `DELETE FROM gmail_automation_rules WHERE id = $1;`

	cmdTag, err := s.db.Exec(ctx, query, ruleID)
	if err != nil {
		s.log.Error("Failed to delete rule", zap.Error(err), zap.String("rule_id", ruleID.String()))
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		s.log.Warn("No rule found to delete", zap.String("rule_id", ruleID.String()))
		return errors.New("no Gmail rule found with ID " + ruleID.String() + " to delete")
	}

	s.log.Info("Successfully deleted rule", zap.String("rule_id", ruleID.String()))
	return nil
}

// ToggleGmailRuleStatus toggles the active status of a Gmail automation rule
func (s *GmailStore) ToggleGmailRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.GmailAutomationRule, error) {
	query := `
		UPDATE gmail_automation_rules
		SET is_active = NOT is_active, updated_at = now()
		WHERE id = $1
		RETURNING id, connected_account_id, name, description, is_active, trigger_type,
		          trigger_conditions, action_type, action_params, priority, created_at, updated_at;
	`

	row := s.db.QueryRow(ctx, query, ruleID)

	var rule domain.GmailAutomationRule
	err := row.Scan(
		&rule.ID, &rule.ConnectedAccountID, &rule.Name, &rule.Description, &rule.IsActive,
		&rule.TriggerType, &rule.TriggerConditions, &rule.ActionType, &rule.ActionParams,
		&rule.Priority, &rule.CreatedAt, &rule.UpdatedAt,
	)

	if err != nil {
		return domain.GmailAutomationRule{}, err
	}

	return rule, nil
}

// StoreGmailMessage stores or updates a Gmail message
func (s *GmailStore) StoreGmailMessage(ctx context.Context, arg StoreGmailMessageParams) error {
	query := `
		INSERT INTO gmail_messages (
			connected_account_id, gmail_message_id, gmail_thread_id, subject, sender,
			recipients, cc_recipients, bcc_recipients, snippet, status, is_starred,
			has_attachments, attachment_count, size_estimate, received_at, labels
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (connected_account_id, gmail_message_id)
		DO UPDATE SET
			subject = EXCLUDED.subject,
			sender = EXCLUDED.sender,
			recipients = EXCLUDED.recipients,
			cc_recipients = EXCLUDED.cc_recipients,
			bcc_recipients = EXCLUDED.bcc_recipients,
			snippet = EXCLUDED.snippet,
			status = EXCLUDED.status,
			is_starred = EXCLUDED.is_starred,
			has_attachments = EXCLUDED.has_attachments,
			attachment_count = EXCLUDED.attachment_count,
			size_estimate = EXCLUDED.size_estimate,
			labels = EXCLUDED.labels,
			last_synced = now(),
			updated_at = now();
	`

	_, err := s.db.Exec(ctx, query,
		arg.ConnectedAccountID, arg.GmailMessageID, arg.GmailThreadID, arg.Subject, arg.Sender,
		arg.Recipients, arg.CcRecipients, arg.BccRecipients, arg.Snippet, arg.Status, arg.IsStarred,
		arg.HasAttachments, arg.AttachmentCount, arg.SizeEstimate, arg.ReceivedAt, arg.Labels,
	)

	if err != nil {
		return err
	}

	return nil
}

// StoreGmailThread stores or updates a Gmail thread
func (s *GmailStore) StoreGmailThread(ctx context.Context, arg StoreGmailThreadParams) error {
	query := `
		INSERT INTO gmail_threads (
			connected_account_id, gmail_thread_id, subject, snippet, message_count,
			has_unread, last_message_at, labels
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (connected_account_id, gmail_thread_id)
		DO UPDATE SET
			subject = EXCLUDED.subject,
			snippet = EXCLUDED.snippet,
			message_count = EXCLUDED.message_count,
			has_unread = EXCLUDED.has_unread,
			last_message_at = EXCLUDED.last_message_at,
			labels = EXCLUDED.labels,
			last_synced = now(),
			updated_at = now();
	`

	_, err := s.db.Exec(ctx, query,
		arg.ConnectedAccountID, arg.GmailThreadID, arg.Subject, arg.Snippet,
		arg.MessageCount, arg.HasUnread, arg.LastMessageAt, arg.Labels,
	)

	if err != nil {
		return err
	}

	return nil
}

// UpdateGmailMessageStatus updates the status of a Gmail message.
func (s *GmailStore) UpdateGmailMessageStatus(
	ctx context.Context,
	accountID uuid.UUID,
	messageID string,
	status domain.GmailMessageStatus,
) error {
	query := `
		UPDATE gmail_messages
		SET status = $1, updated_at = now()
		WHERE connected_account_id = $2 AND gmail_message_id = $3;
	`

	cmdTag, err := s.db.Exec(ctx, query, status, accountID, messageID)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return errors.New("no message found with ID " + messageID + " for account " + accountID.String())
	}

	return nil
}

// GetGmailMessagesForAccount gets recent Gmail messages for an account.
func (s *GmailStore) GetGmailMessagesForAccount(
	ctx context.Context,
	accountID uuid.UUID,
	limit int,
) ([]domain.GmailMessage, error) {
	query := `
		SELECT id, connected_account_id, gmail_message_id, gmail_thread_id, subject, sender,
		       recipients, cc_recipients, bcc_recipients, snippet, status, is_starred,
		       has_attachments, attachment_count, size_estimate, received_at, labels,
		       last_synced, created_at, updated_at
		FROM gmail_messages
		WHERE connected_account_id = $1
		ORDER BY received_at DESC
		LIMIT $2;
	`

	rows, err := s.db.Query(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []domain.GmailMessage
	for rows.Next() {
		var msg domain.GmailMessage
		err := rows.Scan(
			&msg.ID, &msg.ConnectedAccountID, &msg.GmailMessageID, &msg.GmailThreadID,
			&msg.Subject, &msg.Sender, &msg.Recipients, &msg.CcRecipients, &msg.BccRecipients,
			&msg.Snippet, &msg.Status, &msg.IsStarred, &msg.HasAttachments, &msg.AttachmentCount,
			&msg.SizeEstimate, &msg.ReceivedAt, &msg.Labels, &msg.LastSynced, &msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// UpdateGmailSyncState updates the Gmail sync state for an account.
func (s *GmailStore) UpdateGmailSyncState(
	ctx context.Context,
	accountID uuid.UUID,
	historyID string,
	lastSync time.Time,
) error {
	query := `
		UPDATE connected_accounts
		SET gmail_history_id = $1, gmail_last_sync = $2, updated_at = now()
		WHERE id = $3;
	`

	_, err := s.db.Exec(ctx, query, historyID, lastSync, accountID)
	if err != nil {
		return err
	}

	return nil
}

// GetGmailSyncState gets the Gmail sync state for an account.
func (s *GmailStore) GetGmailSyncState(
	ctx context.Context,
	accountID uuid.UUID,
) (historyID *string, lastSync *time.Time, err error) {
	query := `
		SELECT gmail_history_id, gmail_last_sync
		FROM connected_accounts
		WHERE id = $1;
	`

	row := s.db.QueryRow(ctx, query, accountID)
	err = row.Scan(&historyID, &lastSync)
	if err != nil {
		if errors.Is(err, errors.New("no rows in result set")) {
			return nil, nil, errors.New("account not found")
		}
		return nil, nil, err
	}

	return historyID, lastSync, nil
}
