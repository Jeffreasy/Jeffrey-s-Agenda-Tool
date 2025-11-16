package store

import (
	"context"
	"time"

	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

// MockStore is a mock implementation of the Storer interface for testing
type MockStore struct {
	mock.Mock
}

// CreateUser mocks the CreateUser method
func (m *MockStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	args := m.Called(ctx, email, name)
	return args.Get(0).(domain.User), args.Error(1)
}

// GetUserByID mocks the GetUserByID method
func (m *MockStore) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.User), args.Error(1)
}

// DeleteUser mocks the DeleteUser method
func (m *MockStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// UpsertConnectedAccount mocks the UpsertConnectedAccount method
func (m *MockStore) UpsertConnectedAccount(ctx context.Context, arg UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.ConnectedAccount), args.Error(1)
}

// GetConnectedAccountByID mocks the GetConnectedAccountByID method
func (m *MockStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.ConnectedAccount), args.Error(1)
}

// GetActiveAccounts mocks the GetActiveAccounts method
func (m *MockStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.ConnectedAccount), args.Error(1)
}

// GetAccountsForUser mocks the GetAccountsForUser method
func (m *MockStore) GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.ConnectedAccount), args.Error(1)
}

// UpdateAccountTokens mocks the UpdateAccountTokens method
func (m *MockStore) UpdateAccountTokens(ctx context.Context, arg UpdateAccountTokensParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

// UpdateAccountLastChecked mocks the UpdateAccountLastChecked method
func (m *MockStore) UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// UpdateAccountStatus mocks the UpdateAccountStatus method
func (m *MockStore) UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

// DeleteConnectedAccount mocks the DeleteConnectedAccount method
func (m *MockStore) DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

// VerifyAccountOwnership mocks the VerifyAccountOwnership method
func (m *MockStore) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, accountID, userID)
	return args.Error(0)
}

// CreateAutomationRule mocks the CreateAutomationRule method
func (m *MockStore) CreateAutomationRule(ctx context.Context, arg CreateAutomationRuleParams) (domain.AutomationRule, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}

// GetRuleByID mocks the GetRuleByID method
func (m *MockStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	args := m.Called(ctx, ruleID)
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}

// GetRulesForAccount mocks the GetRulesForAccount method
func (m *MockStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).([]domain.AutomationRule), args.Error(1)
}

// UpdateRule mocks the UpdateRule method
func (m *MockStore) UpdateRule(ctx context.Context, arg UpdateRuleParams) (domain.AutomationRule, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}

// ToggleRuleStatus mocks the ToggleRuleStatus method
func (m *MockStore) ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	args := m.Called(ctx, ruleID)
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}

// DeleteRule mocks the DeleteRule method
func (m *MockStore) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

// VerifyRuleOwnership mocks the VerifyRuleOwnership method
func (m *MockStore) VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, ruleID, userID)
	return args.Error(0)
}

// CreateAutomationLog mocks the CreateAutomationLog method
func (m *MockStore) CreateAutomationLog(ctx context.Context, arg CreateLogParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

// HasLogForTrigger mocks the HasLogForTrigger method
func (m *MockStore) HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error) {
	args := m.Called(ctx, ruleID, triggerEventID)
	return args.Bool(0), args.Error(1)
}

// GetLogsForAccount mocks the GetLogsForAccount method
func (m *MockStore) GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error) {
	args := m.Called(ctx, accountID, limit)
	return args.Get(0).([]domain.AutomationLog), args.Error(1)
}

// GetValidTokenForAccount mocks the GetValidTokenForAccount method
func (m *MockStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

// UpdateConnectedAccountToken mocks the UpdateConnectedAccountToken method
func (m *MockStore) UpdateConnectedAccountToken(ctx context.Context, params UpdateConnectedAccountTokenParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}

// CreateGmailAutomationRule mocks the CreateGmailAutomationRule method
func (m *MockStore) CreateGmailAutomationRule(ctx context.Context, arg CreateGmailAutomationRuleParams) (domain.GmailAutomationRule, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.GmailAutomationRule), args.Error(1)
}

// GetGmailRulesForAccount mocks the GetGmailRulesForAccount method
func (m *MockStore) GetGmailRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GmailAutomationRule, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).([]domain.GmailAutomationRule), args.Error(1)
}

// UpdateGmailRule mocks the UpdateGmailRule method
func (m *MockStore) UpdateGmailRule(ctx context.Context, arg UpdateGmailRuleParams) (domain.GmailAutomationRule, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.GmailAutomationRule), args.Error(1)
}

// DeleteGmailRule mocks the DeleteGmailRule method
func (m *MockStore) DeleteGmailRule(ctx context.Context, ruleID uuid.UUID) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

// ToggleGmailRuleStatus mocks the ToggleGmailRuleStatus method
func (m *MockStore) ToggleGmailRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.GmailAutomationRule, error) {
	args := m.Called(ctx, ruleID)
	return args.Get(0).(domain.GmailAutomationRule), args.Error(1)
}

// StoreGmailMessage mocks the StoreGmailMessage method
func (m *MockStore) StoreGmailMessage(ctx context.Context, arg StoreGmailMessageParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

// StoreGmailThread mocks the StoreGmailThread method
func (m *MockStore) StoreGmailThread(ctx context.Context, arg StoreGmailThreadParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

// UpdateGmailMessageStatus mocks the UpdateGmailMessageStatus method
func (m *MockStore) UpdateGmailMessageStatus(ctx context.Context, accountID uuid.UUID, messageID string, status domain.GmailMessageStatus) error {
	args := m.Called(ctx, accountID, messageID, status)
	return args.Error(0)
}

// GetGmailMessagesForAccount mocks the GetGmailMessagesForAccount method
func (m *MockStore) GetGmailMessagesForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.GmailMessage, error) {
	args := m.Called(ctx, accountID, limit)
	return args.Get(0).([]domain.GmailMessage), args.Error(1)
}

// UpdateGmailSyncState mocks the UpdateGmailSyncState method
func (m *MockStore) UpdateGmailSyncState(ctx context.Context, accountID uuid.UUID, historyID string, lastSync time.Time) error {
	args := m.Called(ctx, accountID, historyID, lastSync)
	return args.Error(0)
}

// GetGmailSyncState mocks the GetGmailSyncState method
func (m *MockStore) GetGmailSyncState(ctx context.Context, accountID uuid.UUID) (historyID *string, lastSync *time.Time, err error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(*string), args.Get(1).(*time.Time), args.Error(2)
}

