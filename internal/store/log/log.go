package log

import (
	"agenda-automator-api/internal/domain"
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateLogParams contains parameters for creating automation logs.
type CreateLogParams struct {
	ConnectedAccountID uuid.UUID
	RuleID             uuid.UUID
	Status             domain.AutomationLogStatus
	TriggerDetails     json.RawMessage // []byte
	ActionDetails      json.RawMessage // []byte
	ErrorMessage       string
}

// LogStore handles log-related database operations
type LogStore struct {
	pool *pgxpool.Pool
}

// NewLogStore creates a new LogStore
func NewLogStore(pool *pgxpool.Pool) *LogStore {
	return &LogStore{pool: pool}
}

// CreateAutomationLog creates a new automation log.
func (s *LogStore) CreateAutomationLog(ctx context.Context, arg CreateLogParams) error {
	query := `
    INSERT INTO automation_logs (
        connected_account_id, rule_id, status, trigger_details, action_details, error_message
    ) VALUES ($1, $2, $3, $4, $5, $6);
    `
	_, err := s.pool.Exec(ctx, query,
		arg.ConnectedAccountID,
		arg.RuleID,
		arg.Status,
		arg.TriggerDetails,
		arg.ActionDetails,
		arg.ErrorMessage,
	)
	if err != nil {
		return err
	}
	return nil
}

// HasLogForTrigger checks if a log exists for a trigger event.
func (s *LogStore) HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error) {
	query := `
    SELECT 1
    FROM automation_logs
    WHERE rule_id = $1
      AND status = 'success'
      AND trigger_details->>'google_event_id' = $2
    LIMIT 1;
    `
	var exists int
	err := s.pool.QueryRow(ctx, query, ruleID, triggerEventID).Scan(&exists)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil // Geen log gevonden, dit is geen error
		}
		return false, err // Een chte error
	}

	return true, nil // Gevonden
}

// GetLogsForAccount haalt de meest recente logs op voor een account.
func (s *LogStore) GetLogsForAccount(
	ctx context.Context,
	accountID uuid.UUID,
	limit int,
) ([]domain.AutomationLog, error) {
	query := `
	   SELECT id, connected_account_id, rule_id, timestamp, status,
	          trigger_details, action_details, error_message
	   FROM automation_logs
	   WHERE connected_account_id = $1
	   ORDER BY timestamp DESC
	   LIMIT $2;
	   `

	rows, err := s.pool.Query(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.AutomationLog
	for rows.Next() {
		var log domain.AutomationLog
		err := rows.Scan(
			&log.ID,
			&log.ConnectedAccountID,
			&log.RuleID,
			&log.Timestamp,
			&log.Status,
			&log.TriggerDetails,
			&log.ActionDetails,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}
