package store

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Storer is de interface voor al onze database-interacties.
type Storer interface {
	CreateUser(ctx context.Context, email, name string) (domain.User, error)

	UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) // Aangepast naar Upsert
	GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error)
	GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error)
	UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error
	UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error // NIEUW

	CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error) // NIEUW
	GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error)
}

// DBStore implementeert de Storer interface.
type DBStore struct {
	pool *pgxpool.Pool
}

// NewStore maakt een nieuwe DBStore
func NewStore(pool *pgxpool.Pool) Storer {
	return &DBStore{
		pool: pool,
	}
}

// CreateUser maakt een nieuwe gebruiker aan in de database
func (s *DBStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	query := `
    INSERT INTO users (email, name)
    VALUES ($1, $2)
    ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
    RETURNING id, email, name, created_at, updated_at;
    `

	row := s.pool.QueryRow(ctx, query, email, name)

	var u domain.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		return domain.User{}, fmt.Errorf("db scan error: %w", err)
	}

	return u, nil
}

// UpsertConnectedAccountParams (aangepast van Create)
type UpsertConnectedAccountParams struct {
	UserID         uuid.UUID
	Provider       domain.ProviderType
	Email          string
	ProviderUserID string
	AccessToken    string
	RefreshToken   string
	TokenExpiry    time.Time
	Scopes         []string
}

// UpdateAccountTokensParams ...
type UpdateAccountTokensParams struct {
	AccountID       uuid.UUID
	NewAccessToken  string
	NewRefreshToken string
	NewTokenExpiry  time.Time
}

// UpsertConnectedAccount versleutelt de tokens en slaat het account op (upsert)
func (s *DBStore) UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {

	// Encrypt de tokens
	encryptedAccessToken, err := crypto.Encrypt([]byte(arg.AccessToken))
	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("could not encrypt access token: %w", err)
	}

	var encryptedRefreshToken []byte
	if arg.RefreshToken != "" {
		encryptedRefreshToken, err = crypto.Encrypt([]byte(arg.RefreshToken))
		if err != nil {
			return domain.ConnectedAccount{}, fmt.Errorf("could not encrypt refresh token: %w", err)
		}
	}

	// Definieer de SQL Query met upsert
	query := `
    INSERT INTO connected_accounts (
        user_id, provider, email, provider_user_id,
        access_token, refresh_token, token_expiry, scopes, status
    ) VALUES (
        $1, $2, $3, $4, $5, $6, $7, $8, 'active'
    )
    ON CONFLICT (user_id, provider, provider_user_id) 
    DO UPDATE SET 
        access_token = EXCLUDED.access_token, 
        refresh_token = EXCLUDED.refresh_token, 
        token_expiry = EXCLUDED.token_expiry, 
        scopes = EXCLUDED.scopes, 
        status = 'active', 
        updated_at = now()
    RETURNING id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status, created_at, updated_at, last_checked;
    `

	row := s.pool.QueryRow(ctx, query,
		arg.UserID,
		arg.Provider,
		arg.Email,
		arg.ProviderUserID,
		encryptedAccessToken,
		encryptedRefreshToken,
		arg.TokenExpiry,
		arg.Scopes,
	)

	var acc domain.ConnectedAccount
	err = row.Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Provider,
		&acc.Email,
		&acc.ProviderUserID,
		&acc.AccessToken,
		&acc.RefreshToken,
		&acc.TokenExpiry,
		&acc.Scopes,
		&acc.Status,
		&acc.CreatedAt,
		&acc.UpdatedAt,
		&acc.LastChecked,
	)

	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)
	}

	return acc, nil
}

// GetConnectedAccountByID ...
func (s *DBStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {
	query := `
        SELECT id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status, created_at, updated_at, last_checked
        FROM connected_accounts
        WHERE id = $1
    `

	row := s.pool.QueryRow(ctx, query, id)

	var acc domain.ConnectedAccount
	err := row.Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Provider,
		&acc.Email,
		&acc.ProviderUserID,
		&acc.AccessToken,
		&acc.RefreshToken,
		&acc.TokenExpiry,
		&acc.Scopes,
		&acc.Status,
		&acc.CreatedAt,
		&acc.UpdatedAt,
		&acc.LastChecked,
	)

	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)
	}

	return acc, nil
}

// UpdateAccountTokens ...
func (s *DBStore) UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error {
	encryptedAccessToken, err := crypto.Encrypt([]byte(arg.NewAccessToken))
	if err != nil {
		return fmt.Errorf("could not encrypt new access token: %w", err)
	}

	var query string
	var args []interface{}

	if arg.NewRefreshToken != "" {
		encryptedRefreshToken, err := crypto.Encrypt([]byte(arg.NewRefreshToken))
		if err != nil {
			return fmt.Errorf("could not encrypt new refresh token: %w", err)
		}

		query = `
        UPDATE connected_accounts
        SET access_token = $1, refresh_token = $2, token_expiry = $3, updated_at = now()
        WHERE id = $4;
        `
		args = []interface{}{encryptedAccessToken, encryptedRefreshToken, arg.NewTokenExpiry, arg.AccountID}
	} else {
		query = `
        UPDATE connected_accounts
        SET access_token = $1, token_expiry = $2, updated_at = now()
        WHERE id = $3;
        `
		args = []interface{}{encryptedAccessToken, arg.NewTokenExpiry, arg.AccountID}
	}

	cmdTag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no account found with ID %s to update", arg.AccountID)
	}

	return nil
}

// NIEUW: UpdateAccountLastChecked
func (s *DBStore) UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error {
	query := `
    UPDATE connected_accounts
    SET last_checked = now(), updated_at = now()
    WHERE id = $1;
    `

	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}

	return nil
}

// GetActiveAccounts haalt alle accounts op die de worker moet controleren (real-time monitoring)
func (s *DBStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
	query := `
    SELECT id, user_id, provider, email, provider_user_id,
           access_token, refresh_token, token_expiry, scopes, status,
           created_at, updated_at, last_checked
    FROM connected_accounts
    WHERE status = 'active';
    ` // Real-time: Check alle actieve accounts elke keer

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("db query error: %w", err)
	}
	defer rows.Close()

	var accounts []domain.ConnectedAccount
	for rows.Next() {
		var acc domain.ConnectedAccount
		err := rows.Scan(
			&acc.ID,
			&acc.UserID,
			&acc.Provider,
			&acc.Email,
			&acc.ProviderUserID,
			&acc.AccessToken,
			&acc.RefreshToken,
			&acc.TokenExpiry,
			&acc.Scopes,
			&acc.Status,
			&acc.CreatedAt,
			&acc.UpdatedAt,
			&acc.LastChecked,
		)
		if err != nil {
			return nil, fmt.Errorf("db row scan error: %w", err)
		}
		accounts = append(accounts, acc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db rows error: %w", err)
	}

	return accounts, nil
}

// CreateAutomationRuleParams (nieuw)
type CreateAutomationRuleParams struct {
	ConnectedAccountID uuid.UUID
	Name               string
	TriggerConditions  json.RawMessage // []byte
	ActionParams       json.RawMessage // []byte
}

// CreateAutomationRule (nieuw)
func (s *DBStore) CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error) {
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
		return domain.AutomationRule{}, fmt.Errorf("db scan error: %w", err)
	}

	return rule, nil
}

// GetRulesForAccount ...
func (s *DBStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	query := `
    SELECT id, connected_account_id, name, is_active,
           trigger_conditions, action_params, created_at, updated_at
    FROM automation_rules
    WHERE connected_account_id = $1 AND is_active = true;
    `

	rows, err := s.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("db query error: %w", err)
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
			return nil, fmt.Errorf("db row scan error: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db rows error: %w", err)
	}

	return rules, nil
}
