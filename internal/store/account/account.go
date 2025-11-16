package account

import (
	"agenda-automator-api/internal/crypto"
	"agenda-automator-api/internal/domain"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// ErrTokenRevoked wordt gegooid als de gebruiker de toegang heeft ingetrokken.
var ErrTokenRevoked = fmt.Errorf("token access has been revoked by user")

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

// UpdateConnectedAccountTokenParams ...
type UpdateConnectedAccountTokenParams struct {
	ID           uuid.UUID
	AccessToken  []byte
	RefreshToken []byte
	TokenExpiry  time.Time
}

// AccountStore handles account-related database operations
type AccountStore struct {
	pool              *pgxpool.Pool
	googleOAuthConfig *oauth2.Config // Nodig om tokens te verversen
	logger            *zap.Logger
}

// NewAccountStore creates a new AccountStore
func NewAccountStore(pool *pgxpool.Pool, oauthCfg *oauth2.Config, logger *zap.Logger) *AccountStore {
	return &AccountStore{
		pool:              pool,
		googleOAuthConfig: oauthCfg,
		logger:            logger,
	}
}

// UpsertConnectedAccount versleutelt de tokens en slaat het account op (upsert)
func (s *AccountStore) UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {

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
func (s *AccountStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {
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

// UpdateAccountTokens ...
func (s *AccountStore) UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error {
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

// UpdateAccountLastChecked updates the last checked time for an account.
func (s *AccountStore) UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error {
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
func (s *AccountStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
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
func (s *AccountStore) GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) {
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
func (s *AccountStore) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error {
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
func (s *AccountStore) DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error {
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

// UpdateAccountStatus updates the status of an account.
func (s *AccountStore) UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
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

// UpdateConnectedAccountToken update access/refresh token
func (s *AccountStore) UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE connected_accounts
		SET access_token = $1, refresh_token = $2, token_expiry = $3, updated_at = now()
		WHERE id = $4
	`, params.AccessToken, params.RefreshToken, params.TokenExpiry, params.ID)
	return err
}

// getDecryptedToken is een helper om de db struct om te zetten naar een oauth2.Token
func (s *AccountStore) getDecryptedToken(acc domain.ConnectedAccount) (*oauth2.Token, error) {
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
func (s *AccountStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {
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
	s.logger.Info("token expired, refreshing", zap.String("account_id", acc.ID.String()), zap.String("user_id", acc.UserID.String()), zap.String("component", "store"))

	// BELANGRIJK: Gebruik context.Background() voor de externe refresh-call.
	// De 'ctx' van de caller kan de 'Authorization' header van de API request bevatten,
	// wat de refresh call van oauth2 in de war brengt.
	cleanCtx := context.Background()

	ts := s.googleOAuthConfig.TokenSource(cleanCtx, token) // <-- Gebruik cleanCtx
	newToken, err := ts.Token()
	if err != nil {
		// Vang 'invalid_grant'
		if strings.Contains(err.Error(), "invalid_grant") {
			s.logger.Error("access revoked for account, setting status to revoked", zap.String("account_id", acc.ID.String()), zap.String("component", "store"))
			if err := s.UpdateAccountStatus(ctx, acc.ID, domain.StatusRevoked); err != nil {
				s.logger.Error("failed to update status for revoked account", zap.Error(err), zap.String("account_id", acc.ID.String()), zap.String("component", "store"))
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

	s.logger.Info("token successfully refreshed and saved", zap.String("account_id", acc.ID.String()), zap.String("component", "store"))

	// 6. Geef het nieuwe, geldige token terug
	return newToken, nil
}
