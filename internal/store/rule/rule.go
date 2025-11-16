package rule

import (
	"context"
	"encoding/json"
	"errors"

	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateAutomationRuleParams contains parameters for creating automation rules.
type CreateAutomationRuleParams struct {
	ConnectedAccountID uuid.UUID
	Name               string
	TriggerConditions  json.RawMessage // []byte
	ActionParams       json.RawMessage // []byte
}

// UpdateRuleParams definieert de parameters voor het bijwerken van een regel.
type UpdateRuleParams struct {
	RuleID            uuid.UUID
	Name              string
	TriggerConditions json.RawMessage
	ActionParams      json.RawMessage
}

// RuleStore handles rule-related database operations
type RuleStore struct {
	pool *pgxpool.Pool
}

// NewRuleStore creates a new RuleStore
func NewRuleStore(pool *pgxpool.Pool) *RuleStore {
	return &RuleStore{pool: pool}
}

// scanRule scans a database row into an AutomationRule
func scanRule(row pgx.Row) (domain.AutomationRule, error) {
	var rule domain.AutomationRule
	err := row.Scan(
		&rule.ID,
		&rule.ConnectedAccountID,
		&rule.Name,
		&rule.IsActive,
		&rule.TriggerConditions,
		&rule.ActionParams,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	return rule, err
}

// CreateAutomationRule creates a new automation rule.
func (s *RuleStore) CreateAutomationRule(
	ctx context.Context,
	arg CreateAutomationRuleParams,
) (domain.AutomationRule, error) {
	query := `
    INSERT INTO automation_rules (
        connected_account_id, name, trigger_conditions, action_params
    ) VALUES (
        $1, $2, $3, $4
    )
    RETURNING id, connected_account_id, name, is_active, trigger_conditions, action_params, created_at, updated_at;
    `

	row := s.pool.QueryRow(ctx, query,
		arg.ConnectedAccountID,
		arg.Name,
		arg.TriggerConditions,
		arg.ActionParams,
	)

	return scanRule(row)
}

// GetRuleByID ...
func (s *RuleStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	query := `
	    SELECT id, connected_account_id, name, is_active,
	           trigger_conditions, action_params, created_at, updated_at
	    FROM automation_rules
	    WHERE id = $1
	    `
	row := s.pool.QueryRow(ctx, query, ruleID)

	var rule domain.AutomationRule
	err := row.Scan(
		&rule.ID,
		&rule.ConnectedAccountID,
		&rule.Name,
		&rule.IsActive,
		&rule.TriggerConditions,
		&rule.ActionParams,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AutomationRule{}, errors.New("rule not found")
		}
		return domain.AutomationRule{}, err
	}

	return rule, nil
}

// GetRulesForAccount ...
func (s *RuleStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	query := `
    SELECT id, connected_account_id, name, is_active,
           trigger_conditions, action_params, created_at, updated_at
    FROM automation_rules
    WHERE connected_account_id = $1
    ORDER BY created_at DESC;
    `

	rows, err := s.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.AutomationRule
	for rows.Next() {
		var rule domain.AutomationRule
		err := rows.Scan(
			&rule.ID,
			&rule.ConnectedAccountID,
			&rule.Name,
			&rule.IsActive,
			&rule.TriggerConditions,
			&rule.ActionParams,
			&rule.CreatedAt,
			&rule.UpdatedAt,
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

// UpdateRule werkt een bestaande regel bij.
func (s *RuleStore) UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error) {
	query := `
    UPDATE automation_rules
    SET name = $1, trigger_conditions = $2, action_params = $3, updated_at = now()
    WHERE id = $4
    RETURNING id, connected_account_id, name, is_active, trigger_conditions, action_params, created_at, updated_at;
    `
	row := s.pool.QueryRow(ctx, query,
		arg.Name,
		arg.TriggerConditions,
		arg.ActionParams,
		arg.RuleID,
	)

	return scanRule(row)
}

// ToggleRuleStatus zet de 'is_active' boolean van een regel om.
func (s *RuleStore) ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	query := `
    UPDATE automation_rules
    SET is_active = NOT is_active, updated_at = now()
    WHERE id = $1
    RETURNING id, connected_account_id, name, is_active, trigger_conditions, action_params, created_at, updated_at;
    `
	row := s.pool.QueryRow(ctx, query, ruleID)

	var rule domain.AutomationRule
	err := row.Scan(
		&rule.ID,
		&rule.ConnectedAccountID,
		&rule.Name,
		&rule.IsActive,
		&rule.TriggerConditions,
		&rule.ActionParams,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		return domain.AutomationRule{}, err
	}

	return rule, nil
}

// VerifyRuleOwnership controleert of een gebruiker de eigenaar is van de regel (via het account).
func (s *RuleStore) VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error {
	query := `
	   SELECT 1
	   FROM automation_rules r
	   JOIN connected_accounts ca ON r.connected_account_id = ca.id
	   WHERE r.id = $1 AND ca.user_id = $2
	   LIMIT 1;
	   `
	var exists int
	err := s.pool.QueryRow(ctx, query, ruleID, userID).Scan(&exists)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("forbidden: rule not found or does not belong to user")
		}
		return err
	}

	return nil
}

// DeleteRule verwijdert een specifieke regel uit de database.
func (s *RuleStore) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	query := `
	   DELETE FROM automation_rules
	   WHERE id = $1;
	   `

	cmdTag, err := s.pool.Exec(ctx, query, ruleID)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return errors.New("no rule found with ID " + ruleID.String() + " to delete")
	}

	return nil
}
