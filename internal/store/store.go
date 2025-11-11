// Maak dit bestand: internal/store/store.go
package store

import (
	"agenda-automator-api/internal/crypto" // <-- We gebruiken de crypto!
	"agenda-automator-api/internal/domain" // <-- We gebruiken de modellen!
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Storer is de interface voor al onze database-interacties.
// Dit maakt testen (mocking) later makkelijk.
type Storer interface {
	// NIEUW: Functie om een gebruiker aan te maken
	CreateUser(ctx context.Context, email, name string) (domain.User, error)

	// Bestaande functie
	CreateConnectedAccount(ctx context.Context, arg CreateConnectedAccountParams) (domain.ConnectedAccount, error)
	GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error)
	GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error)
	UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error

	// --- NIEUWE FUNCTIE VOOR DE WORKER ---
	GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error)
}

// DBStore implementeert de Storer interface.
// Het bevat de verbinding met de database.
type DBStore struct {
	pool *pgxpool.Pool
}

// NewStore maakt een nieuwe DBStore
func NewStore(pool *pgxpool.Pool) Storer {
	return &DBStore{
		pool: pool,
	}
}

// --- NIEUWE FUNCTIE ---
// CreateUser maakt een nieuwe gebruiker aan in de database
func (s *DBStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	query := `
	INSERT INTO users (email, name) 
	VALUES ($1, $2)
	ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
	RETURNING id, email, name, created_at, updated_at;
	`
	// Opmerking: ON CONFLICT (email) DO UPDATE ... is een 'upsert'.
	// Als de gebruiker al bestaat, update het de naam. Zo crasht je test niet
	// als je hem twee keer draait.

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

// --- EINDE NIEUWE FUNCTIE ---

// CreateConnectedAccountParams bevat de data die we nodig hebben van de "buitenwereld"
// om een account aan te maken. We gebruiken NIET direct de 'domain.ConnectedAccount' struct,
// omdat de tokens nog niet versleuteld zijn.
type CreateConnectedAccountParams struct {
	UserID         uuid.UUID
	Provider       domain.ProviderType
	Email          string
	ProviderUserID string
	AccessToken    string // Let op: string!
	RefreshToken   string // Let op: string!
	TokenExpiry    time.Time
	Scopes         []string
}

// UpdateAccountTokensParams bevat de data om een token te vernieuwen
type UpdateAccountTokensParams struct {
	AccountID       uuid.UUID
	NewAccessToken  string // platte tekst
	NewRefreshToken string // platte tekst (optioneel, soms krijg je geen nieuwe)
	NewTokenExpiry  time.Time
}

// CreateConnectedAccount versleutelt de tokens en slaat het account op in de DB.
func (s *DBStore) CreateConnectedAccount(ctx context.Context, arg CreateConnectedAccountParams) (domain.ConnectedAccount, error) {

	// 1. Encrypt de tokens
	encryptedAccessToken, err := crypto.Encrypt([]byte(arg.AccessToken))
	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("could not encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := crypto.Encrypt([]byte(arg.RefreshToken))
	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("could not encrypt refresh token: %w", err)
	}

	// 2. Definieer de SQL Query
	query := `
	INSERT INTO connected_accounts (
		user_id, provider, email, provider_user_id, 
		access_token, refresh_token, token_expiry, scopes, status
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, 'active'
	)
	RETURNING id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status, created_at, updated_at;
	`

	// 3. Voer de query uit
	// We gebruiken QueryRow omdat we één rij terug verwachten (RETURNING ...)
	row := s.pool.QueryRow(ctx, query,
		arg.UserID,
		arg.Provider,
		arg.Email,
		arg.ProviderUserID,
		encryptedAccessToken,  // <-- Het versleutelde []byte
		encryptedRefreshToken, // <-- Het versleutelde []byte
		arg.TokenExpiry,
		arg.Scopes,
	)

	// 4. Scan de teruggegeven rij in onze struct
	var acc domain.ConnectedAccount
	err = row.Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Provider,
		&acc.Email,
		&acc.ProviderUserID,
		&acc.AccessToken,  // <-- Dit is nu de versleutelde []byte
		&acc.RefreshToken, // <-- Dit is nu de versleutelde []byte
		&acc.TokenExpiry,
		&acc.Scopes,
		&acc.Status,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)

	if err != nil {
		// Hier kun je specifieke db errors afvangen (bijv. 'unique constraint')
		return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)
	}

	return acc, nil
}

// GetConnectedAccountByID haalt één specifiek account op uit de database op basis van ID.
// De tokens zijn nog versleuteld ([]byte).
func (s *DBStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {
	query := `
		SELECT id, user_id, provider, email, provider_user_id, access_token, refresh_token, token_expiry, scopes, status, created_at, updated_at
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
	)

	if err != nil {
		return domain.ConnectedAccount{}, fmt.Errorf("db scan error: %w", err)
	}

	return acc, nil
}

func (s *DBStore) UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error {
	// 1. Encrypt de nieuwe tokens
	encryptedAccessToken, err := crypto.Encrypt([]byte(arg.NewAccessToken))
	if err != nil {
		return fmt.Errorf("could not encrypt new access token: %w", err)
	}

	// 2. Bouw de query. We updaten de refresh token alleen als er een nieuwe is.
	var query string
	var args []interface{}

	if arg.NewRefreshToken != "" {
		// Encrypt de nieuwe refresh token
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
		// Update alleen het access token
		query = `
		UPDATE connected_accounts
		SET access_token = $1, token_expiry = $2, updated_at = now()
		WHERE id = $3;
		`
		args = []interface{}{encryptedAccessToken, arg.NewTokenExpiry, arg.AccountID}
	}

	// 3. Voer de update uit
	cmdTag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("db exec error: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no account found with ID %s to update", arg.AccountID)
	}

	return nil
}

// --- NIEUWE FUNCTIE: GetActiveAccounts ---
// Haalt alle accounts op die de worker moet controleren
func (s *DBStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
	query := `
	SELECT id, user_id, provider, email, provider_user_id,
		   access_token, refresh_token, token_expiry, scopes, status,
		   created_at, updated_at
	FROM connected_accounts
	WHERE status = 'active';
	` // We controleren alleen 'actieve' accounts

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

// --- NIEUWE FUNCTIE: GetRulesForAccount ---
// Haalt alle actieve regels op voor een specifiek account
func (s *DBStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	query := `
	SELECT id, connected_account_id, name, is_active,
		   trigger_conditions, action_params, created_at, updated_at
	FROM automation_rules
	WHERE connected_account_id = $1 AND is_active = true;
	` // We gebruiken hier de 'partial index' die we hebben gemaakt!

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
			&rule.TriggerConditions, // Dit is []byte
			&rule.ActionParams,      // Dit is []byte
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
