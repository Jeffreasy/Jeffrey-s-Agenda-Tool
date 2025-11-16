package store

import (
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store/account"
	"agenda-automator-api/internal/store/gmail"
	"agenda-automator-api/internal/store/log"
	"agenda-automator-api/internal/store/rule"
	"agenda-automator-api/internal/store/user"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Re-export parameter structs for backward compatibility
type (
	UpsertConnectedAccountParams      = account.UpsertConnectedAccountParams
	UpdateAccountTokensParams         = account.UpdateAccountTokensParams
	UpdateConnectedAccountTokenParams = account.UpdateConnectedAccountTokenParams
	CreateAutomationRuleParams        = rule.CreateAutomationRuleParams
	UpdateRuleParams                  = rule.UpdateRuleParams
	CreateLogParams                   = log.CreateLogParams
	CreateGmailAutomationRuleParams   = gmail.CreateGmailAutomationRuleParams
	UpdateGmailRuleParams             = gmail.UpdateGmailRuleParams
	StoreGmailMessageParams           = gmail.StoreGmailMessageParams
	StoreGmailThreadParams            = gmail.StoreGmailThreadParams
)

// ErrTokenRevoked re-export error for backward compatibility
var ErrTokenRevoked = account.ErrTokenRevoked

// Storer is de interface voor al onze database-interactions.
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

	// Gmail-specific methods
	CreateGmailAutomationRule(ctx context.Context, arg CreateGmailAutomationRuleParams) (domain.GmailAutomationRule, error)
	GetGmailRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GmailAutomationRule, error)
	UpdateGmailRule(ctx context.Context, arg UpdateGmailRuleParams) (domain.GmailAutomationRule, error)
	DeleteGmailRule(ctx context.Context, ruleID uuid.UUID) error
	ToggleGmailRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.GmailAutomationRule, error)

	// Gmail message storage
	StoreGmailMessage(ctx context.Context, arg StoreGmailMessageParams) error
	StoreGmailThread(ctx context.Context, arg StoreGmailThreadParams) error
	UpdateGmailMessageStatus(ctx context.Context, accountID uuid.UUID, messageID string, status domain.GmailMessageStatus) error
	GetGmailMessagesForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.GmailMessage, error)

	// Gmail sync tracking
	UpdateGmailSyncState(ctx context.Context, accountID uuid.UUID, historyID string, lastSync time.Time) error
	GetGmailSyncState(ctx context.Context, accountID uuid.UUID) (historyID *string, lastSync *time.Time, err error)
}

// DBStore implementeert de Storer interface.
type DBStore struct {
	userStore    *user.UserStore
	accountStore *account.AccountStore
	ruleStore    *rule.RuleStore
	logStore     *log.LogStore
	gmailStore   *gmail.GmailStore
}

// NewStore maakt een nieuwe DBStore
func NewStore(pool *pgxpool.Pool, oauthCfg *oauth2.Config, logger *zap.Logger) Storer {
	return &DBStore{
		userStore:    user.NewUserStore(pool),
		accountStore: account.NewAccountStore(pool, oauthCfg, logger),
		ruleStore:    rule.NewRuleStore(pool),
		logStore:     log.NewLogStore(pool),
		gmailStore:   gmail.NewGmailStore(pool, logger),
	}
}

// --- USER FUNCTIES ---

// CreateUser maakt een nieuwe gebruiker aan in de database
func (s *DBStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	return s.userStore.CreateUser(ctx, email, name)
}

// GetUserByID haalt een gebruiker op basis van ID.
func (s *DBStore) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.userStore.GetUserByID(ctx, userID)
}

// DeleteUser verwijdert een gebruiker en al zijn data (via ON DELETE CASCADE).
func (s *DBStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return s.userStore.DeleteUser(ctx, userID)
}

// --- ACCOUNT FUNCTIES ---

// UpsertConnectedAccount versleutelt de tokens en slaat het account op (upsert)
func (s *DBStore) UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {
	return s.accountStore.UpsertConnectedAccount(ctx, arg)
}

// GetConnectedAccountByID ...
func (s *DBStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {
	return s.accountStore.GetConnectedAccountByID(ctx, id)
}

// UpdateAccountTokens ...
func (s *DBStore) UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error {
	return s.accountStore.UpdateAccountTokens(ctx, arg)
}

// UpdateAccountLastChecked updates the last checked time for an account.
func (s *DBStore) UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error {
	return s.accountStore.UpdateAccountLastChecked(ctx, id)
}

// GetActiveAccounts haalt alle accounts op die de worker moet controleren
func (s *DBStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
	return s.accountStore.GetActiveAccounts(ctx)
}

// GetAccountsForUser haalt alle accounts op die eigendom zijn van een specifieke gebruiker
func (s *DBStore) GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) {
	return s.accountStore.GetAccountsForUser(ctx, userID)
}

// VerifyAccountOwnership controleert of een gebruiker eigenaar is van een account
func (s *DBStore) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error {
	return s.accountStore.VerifyAccountOwnership(ctx, accountID, userID)
}

// DeleteConnectedAccount verwijdert een specifiek account en diens data.
func (s *DBStore) DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error {
	return s.accountStore.DeleteConnectedAccount(ctx, accountID)
}

// --- RULE FUNCTIES ---

// CreateAutomationRule creates a new automation rule.
func (s *DBStore) CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error) {
	return s.ruleStore.CreateAutomationRule(ctx, arg)
}

// GetRuleByID ...
func (s *DBStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	return s.ruleStore.GetRuleByID(ctx, ruleID)
}

// GetRulesForAccount ...
func (s *DBStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	return s.ruleStore.GetRulesForAccount(ctx, accountID)
}

// UpdateRule werkt een bestaande regel bij.
func (s *DBStore) UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error) {
	return s.ruleStore.UpdateRule(ctx, arg)
}

// ToggleRuleStatus zet de 'is_active' boolean van een regel om.
func (s *DBStore) ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	return s.ruleStore.ToggleRuleStatus(ctx, ruleID)
}

// VerifyRuleOwnership controleert of een gebruiker de eigenaar is van de regel (via het account).
func (s *DBStore) VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error {
	return s.ruleStore.VerifyRuleOwnership(ctx, ruleID, userID)
}

// DeleteRule verwijdert een specifieke regel uit de database.
func (s *DBStore) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	return s.ruleStore.DeleteRule(ctx, ruleID)
}

// --- LOG FUNCTIES ---

// UpdateAccountStatus updates the status of an account.
func (s *DBStore) UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	return s.accountStore.UpdateAccountStatus(ctx, id, status)
}

// CreateAutomationLog creates a new automation log.
func (s *DBStore) CreateAutomationLog(ctx context.Context, arg CreateLogParams) error {
	return s.logStore.CreateAutomationLog(ctx, arg)
}

// HasLogForTrigger checks if a log exists for a trigger event.
func (s *DBStore) HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error) {
	return s.logStore.HasLogForTrigger(ctx, ruleID, triggerEventID)
}

// GetLogsForAccount haalt de meest recente logs op voor een account.
func (s *DBStore) GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error) {
	return s.logStore.GetLogsForAccount(ctx, accountID, limit)
}

// --- GECENTRALISEERDE TOKEN LOGICA ---

// GetValidTokenForAccount is de centrale functie die een token ophaalt,
// en indien nodig ververst en opslaat.
func (s *DBStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {
	return s.accountStore.GetValidTokenForAccount(ctx, accountID)
}

// UpdateConnectedAccountToken update access/refresh token
func (s *DBStore) UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error {
	return s.accountStore.UpdateConnectedAccountToken(ctx, params)
}

// --- GMAIL AUTOMATION RULE METHODS ---

// CreateGmailAutomationRule creates a new Gmail automation rule
func (s *DBStore) CreateGmailAutomationRule(ctx context.Context, arg CreateGmailAutomationRuleParams) (domain.GmailAutomationRule, error) {
	return s.gmailStore.CreateGmailAutomationRule(ctx, arg)
}

// GetGmailRulesForAccount gets all Gmail automation rules for an account
func (s *DBStore) GetGmailRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GmailAutomationRule, error) {
	return s.gmailStore.GetGmailRulesForAccount(ctx, accountID)
}

// UpdateGmailRule updates an existing Gmail automation rule
func (s *DBStore) UpdateGmailRule(ctx context.Context, arg UpdateGmailRuleParams) (domain.GmailAutomationRule, error) {
	return s.gmailStore.UpdateGmailRule(ctx, arg)
}

// DeleteGmailRule deletes a Gmail automation rule
func (s *DBStore) DeleteGmailRule(ctx context.Context, ruleID uuid.UUID) error {
	return s.gmailStore.DeleteGmailRule(ctx, ruleID)
}

// ToggleGmailRuleStatus toggles the active status of a Gmail automation rule
func (s *DBStore) ToggleGmailRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.GmailAutomationRule, error) {
	return s.gmailStore.ToggleGmailRuleStatus(ctx, ruleID)
}

// --- GMAIL MESSAGE STORAGE METHODS ---

// StoreGmailMessage stores or updates a Gmail message
func (s *DBStore) StoreGmailMessage(ctx context.Context, arg StoreGmailMessageParams) error {
	return s.gmailStore.StoreGmailMessage(ctx, arg)
}

// StoreGmailThread stores or updates a Gmail thread
func (s *DBStore) StoreGmailThread(ctx context.Context, arg StoreGmailThreadParams) error {
	return s.gmailStore.StoreGmailThread(ctx, arg)
}

// UpdateGmailMessageStatus updates the status of a Gmail message
func (s *DBStore) UpdateGmailMessageStatus(ctx context.Context, accountID uuid.UUID, messageID string, status domain.GmailMessageStatus) error {
	return s.gmailStore.UpdateGmailMessageStatus(ctx, accountID, messageID, status)
}

// GetGmailMessagesForAccount gets recent Gmail messages for an account
func (s *DBStore) GetGmailMessagesForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.GmailMessage, error) {
	return s.gmailStore.GetGmailMessagesForAccount(ctx, accountID, limit)
}

// --- GMAIL SYNC STATE METHODS ---

// UpdateGmailSyncState updates the Gmail sync state for an account
func (s *DBStore) UpdateGmailSyncState(ctx context.Context, accountID uuid.UUID, historyID string, lastSync time.Time) error {
	return s.gmailStore.UpdateGmailSyncState(ctx, accountID, historyID, lastSync)
}

// GetGmailSyncState gets the Gmail sync state for an account
func (s *DBStore) GetGmailSyncState(ctx context.Context, accountID uuid.UUID) (historyID *string, lastSync *time.Time, err error) {
	return s.gmailStore.GetGmailSyncState(ctx, accountID)
}
