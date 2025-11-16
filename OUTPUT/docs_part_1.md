# Documentation Part 1

Generated at: 2025-11-15T20:45:21+01:00

## internal\store\store.go

```go
package store

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

// ErrTokenRevoked wordt gegooid als de gebruiker de toegang heeft ingetrokken.
var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

// Storer is de interface voor al onze database-interacties.
type Storer interface {
	CreateUser(ctx context.Context, email, name string) (domain.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error)
	DeleteUser(ctx context.Context, userID uuid.UUID) error

	UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error)
	GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error)
	GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error)
	GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error)
	UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error
	UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error
	UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error
	DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error
	VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error

	CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error)
	GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error)
	GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error)
	UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error)
	ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error)
	DeleteRule(ctx context.Context, ruleID uuid.UUID) error
	VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error

	CreateAutomationLog(ctx context.Context, arg CreateLogParams) error
	HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error)
	GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error)

	// Gecentraliseerde Token Logica
	GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error)

	// UpdateConnectedAccountToken update access/refresh token
	UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error
}

// DBStore implementeert de Storer interface.
type DBStore struct {
	pool              *pgxpool.Pool
	googleOAuthConfig *oauth2.Config // Nodig om tokens te verversen
}

// NewStore maakt een nieuwe DBStore
func NewStore(pool *pgxpool.Pool, oauthCfg *oauth2.Config) Storer {
	return &DBStore{
		pool:              pool,
		googleOAuthConfig: oauthCfg,
	}
}

// --- USER FUNCTIES ---

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

// GetUserByID haalt een gebruiker op basis van ID.
func (s *DBStore) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	query := `SELECT id, email, name, created_at, updated_at FROM users WHERE id = $1`
	row := s.pool.QueryRow(ctx, query, userID)

	var u domain.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("user not found")
		}
		return domain.User{}, fmt.Errorf("db scan error: %w", err)
	}

	return u, nil
}

// DeleteUser verwijdert een gebruiker en al zijn data (via ON DELETE CASCADE).
func (s *DBStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	cmdTag, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no user found with ID %s to delete", userID)
	}
	return nil
}

// --- ACCOUNT FUNCTIES ---

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

// UpsertConnectedAccount versleutelt de tokens en slaat het account op (upsert)
func (s *DBStore) UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {

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
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ConnectedAccount{}, fmt.Errorf("account not found")
		}
		return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)
	}

	return acc, nil
}

// UpdateAccountTokensParams ...
type UpdateAccountTokensParams struct {
	AccountID       uuid.UUID
	NewAccessToken  string
	NewRefreshToken string
	NewTokenExpiry  time.Time
}

// UpdateConnectedAccountTokenParams ...
type UpdateConnectedAccountTokenParams struct {
	ID           uuid.UUID
	AccessToken  []byte
	RefreshToken []byte
	TokenExpiry  time.Time
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

// UpdateAccountLastChecked
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

// GetActiveAccounts haalt alle accounts op die de worker moet controleren
func (s *DBStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
	query := `
    SELECT id, user_id, provider, email, provider_user_id,
           access_token, refresh_token, token_expiry, scopes, status,
           created_at, updated_at, last_checked
    FROM connected_accounts
    WHERE status = 'active';
    `

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

// GetAccountsForUser haalt alle accounts op die eigendom zijn van een specifieke gebruiker
func (s *DBStore) GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) {
	query := `
    SELECT id, user_id, provider, email, provider_user_id,
           access_token, refresh_token, token_expiry, scopes, status,
           created_at, updated_at, last_checked
    FROM connected_accounts
    WHERE user_id = $1
    ORDER BY created_at DESC;
    `

	rows, err := s.pool.Query(ctx, query, userID)
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

// VerifyAccountOwnership controleert of een gebruiker eigenaar is van een account
func (s *DBStore) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error {
	query := `
    SELECT 1 
    FROM connected_accounts
    WHERE id = $1 AND user_id = $2
    LIMIT 1;
    `
	var exists int
	err := s.pool.QueryRow(ctx, query, accountID, userID).Scan(&exists)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("forbidden: account not found or does not belong to user")
		}
		return fmt.Errorf("db query error: %w", err)
	}

	return nil
}

// DeleteConnectedAccount verwijdert een specifiek account en diens data.
func (s *DBStore) DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error {
	query := `DELETE FROM connected_accounts WHERE id = $1`
	cmdTag, err := s.pool.Exec(ctx, query, accountID)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no account found with ID %s to delete", accountID)
	}
	return nil
}

// --- RULE FUNCTIES ---

// CreateAutomationRuleParams
type CreateAutomationRuleParams struct {
	ConnectedAccountID uuid.UUID
	Name               string
	TriggerConditions  json.RawMessage // []byte
	ActionParams       json.RawMessage // []byte
}

// CreateAutomationRule
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

// GetRuleByID ...
func (s *DBStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
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
			return domain.AutomationRule{}, fmt.Errorf("rule not found")
		}
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
    WHERE connected_account_id = $1
    ORDER BY created_at DESC;
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

// UpdateRuleParams definieert de parameters voor het bijwerken van een regel.
type UpdateRuleParams struct {
	RuleID            uuid.UUID
	Name              string
	TriggerConditions json.RawMessage
	ActionParams      json.RawMessage
}

// UpdateRule werkt een bestaande regel bij.
func (s *DBStore) UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error) {
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

// ToggleRuleStatus zet de 'is_active' boolean van een regel om.
func (s *DBStore) ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
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
		return domain.AutomationRule{}, fmt.Errorf("db scan error: %w", err)
	}

	return rule, nil
}

// VerifyRuleOwnership controleert of een gebruiker de eigenaar is van de regel (via het account).
func (s *DBStore) VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error {
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
			return fmt.Errorf("forbidden: rule not found or does not belong to user")
		}
		return fmt.Errorf("db query error: %w", err)
	}

	return nil
}

// DeleteRule verwijdert een specifieke regel uit de database.
func (s *DBStore) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	query := `
	   DELETE FROM automation_rules
	   WHERE id = $1;
	   `

	cmdTag, err := s.pool.Exec(ctx, query, ruleID)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no rule found with ID %s to delete", ruleID)
	}

	return nil
}

// --- LOG FUNCTIES ---

// UpdateAccountStatus
func (s *DBStore) UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	query := `
    UPDATE connected_accounts
    SET status = $1, updated_at = now()
    WHERE id = $2;
    `
	_, err := s.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}
	return nil
}

// CreateLogParams
type CreateLogParams struct {
	ConnectedAccountID uuid.UUID
	RuleID             uuid.UUID
	Status             domain.AutomationLogStatus
	TriggerDetails     json.RawMessage // []byte
	ActionDetails      json.RawMessage // []byte
	ErrorMessage       string
}

// CreateAutomationLog
func (s *DBStore) CreateAutomationLog(ctx context.Context, arg CreateLogParams) error {
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
		return fmt.Errorf("db exec error: %w", err)
	}
	return nil
}

// HasLogForTrigger
func (s *DBStore) HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error) {
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
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // Geen log gevonden, dit is geen error
		}
		return false, fmt.Errorf("db query error: %w", err) // Een échte error
	}

	return true, nil // Gevonden
}

// GetLogsForAccount haalt de meest recente logs op voor een account.
func (s *DBStore) GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error) {
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
		return nil, fmt.Errorf("db query error: %w", err)
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
			return nil, fmt.Errorf("db row scan error: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db rows error: %w", err)
	}

	return logs, nil
}

// --- GECENTRALISEERDE TOKEN LOGICA ---

// getDecryptedToken is een helper om de db struct om te zetten naar een oauth2.Token
func (s *DBStore) getDecryptedToken(acc domain.ConnectedAccount) (*oauth2.Token, error) {
	plaintextAccessToken, err := crypto.Decrypt(acc.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt access token: %w", err)
	}

	var plaintextRefreshToken []byte
	if len(acc.RefreshToken) > 0 {
		plaintextRefreshToken, err = crypto.Decrypt(acc.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt refresh token: %w", err)
		}
	}

	return &oauth2.Token{
		AccessToken:  string(plaintextAccessToken),
		RefreshToken: string(plaintextRefreshToken),
		Expiry:       acc.TokenExpiry,
		TokenType:    "Bearer",
	}, nil
}

// GetValidTokenForAccount is de centrale functie die een token ophaalt,
// en indien nodig ververst en opslaat.
func (s *DBStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {
	// 1. Haal account op uit DB
	acc, err := s.GetConnectedAccountByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("kon account niet ophalen: %w", err)
	}

	// 2. Decrypt het token
	token, err := s.getDecryptedToken(acc)
	if err != nil {
		return nil, fmt.Errorf("kon token niet decrypten: %w", err)
	}

	// 3. Controleer of het (bijna) verlopen is
	if token.Valid() {
		return token, nil // Token is prima
	}

	// 4. Token is verlopen, ververs het
	log.Printf("[Store] Token for account %s (User %s) is expired. Refreshing...", acc.ID, acc.UserID)

	// BELANGRIJK: Gebruik context.Background() voor de externe refresh-call.
	// De 'ctx' van de caller kan de 'Authorization' header van de API request bevatten,
	// wat de refresh call van oauth2 in de war brengt.
	cleanCtx := context.Background()

	ts := s.googleOAuthConfig.TokenSource(cleanCtx, token) // <-- Gebruik cleanCtx
	newToken, err := ts.Token()
	if err != nil {
		// Vang 'invalid_grant'
		if strings.Contains(err.Error(), "invalid_grant") {
			log.Printf("[Store] FATAL: Access for account %s has been revoked. Setting status to 'revoked'.", acc.ID)
			if err := s.UpdateAccountStatus(ctx, acc.ID, domain.StatusRevoked); err != nil {
				log.Printf("[Store] ERROR: Failed to update status for revoked account %s: %v", acc.ID, err)
			}
			return nil, ErrTokenRevoked // Gooi specifieke error
		}
		return nil, fmt.Errorf("could not refresh token: %w", err)
	}

	// 5. Sla het nieuwe token op
	// Als we GEEN nieuwe refresh token krijgen, hergebruik dan de oude
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = token.RefreshToken
	}

	err = s.UpdateAccountTokens(ctx, UpdateAccountTokensParams{
		AccountID:       acc.ID,
		NewAccessToken:  newToken.AccessToken,
		NewRefreshToken: newToken.RefreshToken, // Zorg dat we de nieuwe refresh token opslaan
		NewTokenExpiry:  newToken.Expiry,
	})
	if err != nil {
		return nil, fmt.Errorf("kon ververst token niet opslaan: %w", err)
	}

	log.Printf("[Store] Token for account %s successfully refreshed and saved.", acc.ID)

	// 6. Geef het nieuwe, geldige token terug
	return newToken, nil
}

// UpdateConnectedAccountToken update access/refresh token
func (s *DBStore) UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE connected_accounts
		SET access_token = $1, refresh_token = $2, token_expiry = $3, updated_at = now()
		WHERE id = $4
	`, params.AccessToken, params.RefreshToken, params.TokenExpiry, params.ID)
	return err
}

```

## internal\api\server.go

```go
package api

import (
	"agenda-automator-api/internal/store"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type contextKey string

var userContextKey contextKey = "user_id"

type Server struct {
	Router            *chi.Mux
	store             store.Storer
	googleOAuthConfig *oauth2.Config
}

func NewServer(s store.Storer, oauthConfig *oauth2.Config) *Server {
	server := &Server{
		Router:            chi.NewRouter(),
		store:             s,
		googleOAuthConfig: oauthConfig,
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

func (s *Server) setupMiddleware() {
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	s.Router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (s *Server) setupRoutes() {
	s.Router.Route("/api/v1", func(r chi.Router) {
		// Health check (unprotected)
		r.Get("/health", s.handleHealth())

		// Auth routes (bestaand)
		r.Get("/auth/google/login", s.handleGoogleLogin())
		r.Get("/auth/google/callback", s.handleGoogleCallback())

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// User routes (bestaand)
			r.Get("/me", s.handleGetMe())
			r.Get("/users/me", s.handleGetMe())

			// Account routes (bestaand)
			r.Get("/accounts", s.handleGetConnectedAccounts())
			r.Delete("/accounts/{accountId}", s.handleDeleteConnectedAccount())

			// Rule routes (bestaand)
			r.Post("/accounts/{accountId}/rules", s.handleCreateRule())
			r.Get("/accounts/{accountId}/rules", s.handleGetRules())
			r.Put("/rules/{ruleId}", s.handleUpdateRule())
			r.Delete("/rules/{ruleId}", s.handleDeleteRule())
			r.Put("/rules/{ruleId}/toggle", s.handleToggleRule())

			// Log routes (bestaand)
			r.Get("/accounts/{accountId}/logs", s.handleGetAutomationLogs())

			// Calendar routes (bijgewerkt + nieuw)
			r.Get("/accounts/{accountId}/calendar/events", s.handleGetCalendarEvents()) // Bijgewerkt voor multi-calendar

			// NIEUW: CRUD voor events
			r.Post("/accounts/{accountId}/calendar/events", s.handleCreateEvent())
			r.Put("/accounts/{accountId}/calendar/events/{eventId}", s.handleUpdateEvent())
			r.Delete("/accounts/{accountId}/calendar/events/{eventId}", s.handleDeleteEvent())

			// NIEUW: Aggregated events
			r.Post("/calendar/aggregated-events", s.handleGetAggregatedEvents())
		})
	})
}

// authMiddleware valideert JWT en zet user ID in context
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			WriteJSONError(w, http.StatusUnauthorized, "Geen authenticatie header")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("ongeldige signing method")
			}
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			WriteJSONError(w, http.StatusUnauthorized, "Ongeldige token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			WriteJSONError(w, http.StatusUnauthorized, "Ongeldige claims")
			return
		}

		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			WriteJSONError(w, http.StatusUnauthorized, "Geen user ID in token")
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			WriteJSONError(w, http.StatusUnauthorized, "Ongeldig user ID")
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

```

## .env

```bash
#---------------------------------------------------
# 1. APPLICATIE CONFIGURATIE
#---------------------------------------------------
APP_ENV=development

API_PORT=8080

#---------------------------------------------------
# 1.5. DATABASE MIGRATIONS
#---------------------------------------------------
RUN_MIGRATIONS=true

#---------------------------------------------------
# 2. DATABASE (POSTGRES)
#---------------------------------------------------
POSTGRES_USER=postgres
POSTGRES_PASSWORD=Bootje12
POSTGRES_DB=agenda_automator
POSTGRES_HOST=localhost
POSTGRES_PORT=5433

DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

#---------------------------------------------------
# 4. BEVEILIGING & ENCRYPTIE
#---------------------------------------------------
ENCRYPTION_KEY="IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea"

#---------------------------------------------------
# 5. OAUTH CLIENTS (Google)
#---------------------------------------------------
CLIENT_BASE_URL="http://localhost:3000"

OAUTH_REDIRECT_URL="http://localhost:8080/api/v1/auth/google/callback"

GOOGLE_OAUTH_CLIENT_ID="273644756085-9tjakd3cvbkgkct2ttubpv8r9mef3jeh.apps.googleusercontent.com"
GOOGLE_OAUTH_CLIENT_SECRET="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"

# NIEUW: Voor dynamic CORS (GECORRIGEERD)
ALLOWED_ORIGINS=http://localhost:3000,https://prod.com

#---------------------------------------------------
# 6. AUTHENTICATIE (JWT) - (NIEUWE SECTIE)
#---------------------------------------------------
# Genereer een sterke, willekeurige 32-byte sleutel
JWT_SECRET_KEY="een-andere-zeer-sterke-geheime-sleutel"
```

## docker-compose.yml

```yaml
version: '3.8'

services:
  db:
    image: postgres:16-alpine
    container_name: agenda_automator_db
    restart: unless-stopped
    
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: Bootje12
      POSTGRES_DB: agenda_automator
      
    ports:
      - "5433:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      
    healthcheck:
      test: ["CMD-SHELL", "psql -U postgres -d agenda_automator -c 'SELECT 1'"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s

  app:
    build: .
    container_name: agenda_automator_app
    restart: unless-stopped

    ports:
      - "8080:8080"

    depends_on:
      db:
        condition: service_healthy

    environment:
      - DATABASE_URL=postgres://postgres:Bootje12@db:5432/agenda_automator?sslmode=disable
      - APP_ENV=development
      - API_PORT=8080
      - ENCRYPTION_KEY=IJvSU0jEVrm3CBNzdAMoDRT9sQlnZcea
      - CLIENT_BASE_URL=http://localhost:3000
      - OAUTH_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback
      - GOOGLE_OAUTH_CLIENT_ID=${GOOGLE_OAUTH_CLIENT_ID}
      - GOOGLE_OAUTH_CLIENT_SECRET=${GOOGLE_OAUTH_CLIENT_SECRET}
      - RUN_MIGRATIONS=true
      - LOG_MAX_SIZE=10MB
      - ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
      # HIER IS DE TOEVOEGING:
      - JWT_SECRET_KEY=${JWT_SECRET_KEY}

volumes:
  pgdata:
```

## internal\api\json.go

```go
package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON schrijft een standaard JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("could not write json response: %v", err)
	}
}

// WriteJSONError schrijft een standaard JSON error response
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

```

## .gitignore

```gitignore
# Environment variables
.env

# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with `go test -c`
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Logs
*.log

# Database files (for local development)
*.db
*.sqlite

# Temporary files
*.tmp
*.temp
```

## db\migrations\embed.go

```go
// db/migrations/embed.go

package migrations

import "embed"

//go:embed 000001_initial_schema.up.sql
var InitialSchemaUp string

//go:embed 000001_initial_schema.down.sql
var InitialSchemaDown string

// Optioneel: als je ALLE sql-bestanden als een bestandssysteem wilt:
//go:embed *.sql
var SQLFiles embed.FS

```

