package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store/account"
	"agenda-automator-api/internal/store/gmail"
	"agenda-automator-api/internal/store/log"
	"agenda-automator-api/internal/store/rule"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// --- MOCKS VOOR SUB-STORES ---

// MockUserStore (Implementeert nu user.UserStorer)
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) CreateUser(ctx context.Context, email, name string) (domain.User, error) {
	args := m.Called(ctx, email, name)
	if args.Get(0) == nil {
		return domain.User{}, args.Error(1)
	}
	return args.Get(0).(domain.User), args.Error(1)
}
func (m *MockUserStore) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return domain.User{}, args.Error(1)
	}
	return args.Get(0).(domain.User), args.Error(1)
}
func (m *MockUserStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// MockAccountStore (Implementeert nu account.AccountStorer)
type MockAccountStore struct {
	mock.Mock
}

func (m *MockAccountStore) UpsertConnectedAccount(ctx context.Context, arg account.UpsertConnectedAccountParams) (domain.ConnectedAccount, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return domain.ConnectedAccount{}, args.Error(1)
	}
	return args.Get(0).(domain.ConnectedAccount), args.Error(1)
}
func (m *MockAccountStore) GetConnectedAccountByID(ctx context.Context, id uuid.UUID) (domain.ConnectedAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return domain.ConnectedAccount{}, args.Error(1)
	}
	return args.Get(0).(domain.ConnectedAccount), args.Error(1)
}
func (m *MockAccountStore) UpdateAccountTokens(ctx context.Context, arg account.UpdateAccountTokensParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}
func (m *MockAccountStore) UpdateAccountLastChecked(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockAccountStore) GetActiveAccounts(ctx context.Context) ([]domain.ConnectedAccount, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.ConnectedAccount), args.Error(1)
}
func (m *MockAccountStore) GetAccountsForUser(ctx context.Context, userID uuid.UUID) ([]domain.ConnectedAccount, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.ConnectedAccount), args.Error(1)
}
func (m *MockAccountStore) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, accountID, userID)
	return args.Error(0)
}
func (m *MockAccountStore) DeleteConnectedAccount(ctx context.Context, accountID uuid.UUID) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}
func (m *MockAccountStore) UpdateAccountStatus(ctx context.Context, id uuid.UUID, status domain.AccountStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}
func (m *MockAccountStore) UpdateConnectedAccountToken(ctx context.Context, params account.UpdateConnectedAccountTokenParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}
func (m *MockAccountStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

// MockRuleStore (Implementeert nu rule.RuleStorer)
type MockRuleStore struct {
	mock.Mock
}

func (m *MockRuleStore) CreateAutomationRule(ctx context.Context, arg rule.CreateAutomationRuleParams) (domain.AutomationRule, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return domain.AutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}
func (m *MockRuleStore) GetRuleByID(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	args := m.Called(ctx, ruleID)
	if args.Get(0) == nil {
		return domain.AutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}
func (m *MockRuleStore) GetRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.AutomationRule, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AutomationRule), args.Error(1)
}
func (m *MockRuleStore) UpdateRule(ctx context.Context, arg rule.UpdateRuleParams) (domain.AutomationRule, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return domain.AutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}
func (m *MockRuleStore) ToggleRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.AutomationRule, error) {
	args := m.Called(ctx, ruleID)
	if args.Get(0) == nil {
		return domain.AutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.AutomationRule), args.Error(1)
}
func (m *MockRuleStore) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}
func (m *MockRuleStore) VerifyRuleOwnership(ctx context.Context, ruleID uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, ruleID, userID)
	return args.Error(0)
}

// MockLogStore (Implementeert nu log.LogStorer)
type MockLogStore struct {
	mock.Mock
}

func (m *MockLogStore) CreateAutomationLog(ctx context.Context, arg log.CreateLogParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}
func (m *MockLogStore) HasLogForTrigger(ctx context.Context, ruleID uuid.UUID, triggerEventID string) (bool, error) {
	args := m.Called(ctx, ruleID, triggerEventID)
	return args.Bool(0), args.Error(1)
}
func (m *MockLogStore) GetLogsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.AutomationLog, error) {
	args := m.Called(ctx, accountID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AutomationLog), args.Error(1)
}

// MockGmailStore (Implementeert nu gmail.GmailStorer)
type MockGmailStore struct {
	mock.Mock
}

func (m *MockGmailStore) CreateGmailAutomationRule(ctx context.Context, arg gmail.CreateGmailAutomationRuleParams) (domain.GmailAutomationRule, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return domain.GmailAutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.GmailAutomationRule), args.Error(1)
}
func (m *MockGmailStore) GetGmailRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GmailAutomationRule, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.GmailAutomationRule), args.Error(1)
}
func (m *MockGmailStore) UpdateGmailRule(ctx context.Context, arg gmail.UpdateGmailRuleParams) (domain.GmailAutomationRule, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return domain.GmailAutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.GmailAutomationRule), args.Error(1)
}
func (m *MockGmailStore) DeleteGmailRule(ctx context.Context, ruleID uuid.UUID) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}
func (m *MockGmailStore) ToggleGmailRuleStatus(ctx context.Context, ruleID uuid.UUID) (domain.GmailAutomationRule, error) {
	args := m.Called(ctx, ruleID)
	if args.Get(0) == nil {
		return domain.GmailAutomationRule{}, args.Error(1)
	}
	return args.Get(0).(domain.GmailAutomationRule), args.Error(1)
}
func (m *MockGmailStore) StoreGmailMessage(ctx context.Context, arg gmail.StoreGmailMessageParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}
func (m *MockGmailStore) StoreGmailThread(ctx context.Context, arg gmail.StoreGmailThreadParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}
func (m *MockGmailStore) UpdateGmailMessageStatus(ctx context.Context, accountID uuid.UUID, messageID string, status domain.GmailMessageStatus) error {
	args := m.Called(ctx, accountID, messageID, status)
	return args.Error(0)
}
func (m *MockGmailStore) GetGmailMessagesForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.GmailMessage, error) {
	args := m.Called(ctx, accountID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.GmailMessage), args.Error(1)
}
func (m *MockGmailStore) UpdateGmailSyncState(ctx context.Context, accountID uuid.UUID, historyID string, lastSync time.Time) error {
	args := m.Called(ctx, accountID, historyID, lastSync)
	return args.Error(0)
}
func (m *MockGmailStore) GetGmailSyncState(ctx context.Context, accountID uuid.UUID) (*string, *time.Time, error) {
	args := m.Called(ctx, accountID)

	var (
		histID *string
		t      *time.Time
	)

	if args.Get(0) != nil {
		histID = args.Get(0).(*string)
	}
	if args.Get(1) != nil {
		t = args.Get(1).(*time.Time)
	}

	return histID, t, args.Error(2)
}

// --- HULPSTRUCTUUR VOOR TESTS ---

type testStore struct {
	dbStore      *DBStore
	userStore    *MockUserStore
	accountStore *MockAccountStore
	ruleStore    *MockRuleStore
	logStore     *MockLogStore
	gmailStore   *MockGmailStore
}

func newTestStore(_ *testing.T) *testStore {
	mockUser := &MockUserStore{}
	mockAccount := &MockAccountStore{}
	mockRule := &MockRuleStore{}
	mockLog := &MockLogStore{}
	mockGmail := &MockGmailStore{}

	dbStore := &DBStore{
		userStore:    mockUser,
		accountStore: mockAccount,
		ruleStore:    mockRule,
		logStore:     mockLog,
		gmailStore:   mockGmail, // <-- Dit zal nu correct werken
	}

	return &testStore{
		dbStore:      dbStore,
		userStore:    mockUser,
		accountStore: mockAccount,
		ruleStore:    mockRule,
		logStore:     mockLog,
		gmailStore:   mockGmail,
	}
}

// --- TESTS ---

func TestNewStore(t *testing.T) {
	testLogger := zap.NewNop()
	store := NewStore(nil, nil, testLogger)
	assert.NotNil(t, store)

	dbStore, ok := store.(*DBStore)
	assert.True(t, ok)
	assert.NotNil(t, dbStore)
}

func TestNewStore_WithParameters(t *testing.T) {
	pool := &pgxpool.Pool{}
	oauthConfig := &oauth2.Config{}
	testLogger := zap.NewNop()

	store := NewStore(pool, oauthConfig, testLogger)
	assert.NotNil(t, store)
}

func TestDBStore_ImplementsStorerInterface(t *testing.T) {
	var store Storer = &DBStore{}
	assert.NotNil(t, store)
}

func TestDBStore_UserMethods(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	ts := newTestStore(t)

	// Test CreateUser
	expectedUser := domain.User{} // <-- GEWIJZIGD: Lege struct
	ts.userStore.On("CreateUser", ctx, "test@example.com", "Test User").Return(expectedUser, nil)
	user, err := ts.dbStore.CreateUser(ctx, "test@example.com", "Test User")
	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	ts.userStore.AssertExpectations(t)

	// Test GetUserByID
	ts.userStore.On("GetUserByID", ctx, userID).Return(expectedUser, nil)
	user, err = ts.dbStore.GetUserByID(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	ts.userStore.AssertExpectations(t)

	// Test DeleteUser
	ts.userStore.On("DeleteUser", ctx, userID).Return(nil)
	err = ts.dbStore.DeleteUser(ctx, userID)
	assert.NoError(t, err)
	ts.userStore.AssertExpectations(t)
}

func TestDBStore_AccountMethods(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	userID := uuid.New()
	expectedAccount := domain.ConnectedAccount{} // <-- GEWIJZIGD
	ts := newTestStore(t)

	// Test UpsertConnectedAccount
	upsertParams := UpsertConnectedAccountParams{}
	ts.accountStore.On("UpsertConnectedAccount", ctx, upsertParams).Return(expectedAccount, nil)
	acc, err := ts.dbStore.UpsertConnectedAccount(ctx, upsertParams)
	assert.NoError(t, err)
	assert.Equal(t, expectedAccount, acc)

	// Test GetConnectedAccountByID
	ts.accountStore.On("GetConnectedAccountByID", ctx, accountID).Return(expectedAccount, nil)
	acc, err = ts.dbStore.GetConnectedAccountByID(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, expectedAccount, acc)

	// Test UpdateAccountTokens
	tokenParams := UpdateAccountTokensParams{}
	ts.accountStore.On("UpdateAccountTokens", ctx, tokenParams).Return(nil)
	err = ts.dbStore.UpdateAccountTokens(ctx, tokenParams)
	assert.NoError(t, err)

	// Test UpdateAccountLastChecked
	ts.accountStore.On("UpdateAccountLastChecked", ctx, accountID).Return(nil)
	err = ts.dbStore.UpdateAccountLastChecked(ctx, accountID)
	assert.NoError(t, err)

	// Test GetActiveAccounts
	expectedAccounts := []domain.ConnectedAccount{expectedAccount}
	ts.accountStore.On("GetActiveAccounts", ctx).Return(expectedAccounts, nil)
	accs, err := ts.dbStore.GetActiveAccounts(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedAccounts, accs)

	// Test GetAccountsForUser
	ts.accountStore.On("GetAccountsForUser", ctx, userID).Return(expectedAccounts, nil)
	accs, err = ts.dbStore.GetAccountsForUser(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, expectedAccounts, accs)

	// Test VerifyAccountOwnership
	ts.accountStore.On("VerifyAccountOwnership", ctx, accountID, userID).Return(nil)
	err = ts.dbStore.VerifyAccountOwnership(ctx, accountID, userID)
	assert.NoError(t, err)

	// Test DeleteConnectedAccount
	ts.accountStore.On("DeleteConnectedAccount", ctx, accountID).Return(nil)
	err = ts.dbStore.DeleteConnectedAccount(ctx, accountID)
	assert.NoError(t, err)

	// Test UpdateAccountStatus
	ts.accountStore.On("UpdateAccountStatus", ctx, accountID, domain.StatusActive).Return(nil)
	err = ts.dbStore.UpdateAccountStatus(ctx, accountID, domain.StatusActive)
	assert.NoError(t, err)

	// Test GetValidTokenForAccount
	expectedToken := &oauth2.Token{}
	ts.accountStore.On("GetValidTokenForAccount", ctx, accountID).Return(expectedToken, nil)
	tok, err := ts.dbStore.GetValidTokenForAccount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, expectedToken, tok)

	// Test UpdateConnectedAccountToken
	updateTokenParams := UpdateConnectedAccountTokenParams{}
	ts.accountStore.On("UpdateConnectedAccountToken", ctx, updateTokenParams).Return(nil)
	err = ts.dbStore.UpdateConnectedAccountToken(ctx, updateTokenParams)
	assert.NoError(t, err)

	ts.accountStore.AssertExpectations(t)
}

func TestDBStore_RuleMethods(t *testing.T) {
	ctx := context.Background()
	ruleID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()
	expectedRule := domain.AutomationRule{} // <-- GEWIJZIGD
	ts := newTestStore(t)

	// Test CreateAutomationRule
	createParams := CreateAutomationRuleParams{}
	ts.ruleStore.On("CreateAutomationRule", ctx, createParams).Return(expectedRule, nil)
	rule, err := ts.dbStore.CreateAutomationRule(ctx, createParams)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test GetRuleByID
	ts.ruleStore.On("GetRuleByID", ctx, ruleID).Return(expectedRule, nil)
	rule, err = ts.dbStore.GetRuleByID(ctx, ruleID)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test GetRulesForAccount
	expectedRules := []domain.AutomationRule{expectedRule}
	ts.ruleStore.On("GetRulesForAccount", ctx, accountID).Return(expectedRules, nil)
	rules, err := ts.dbStore.GetRulesForAccount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, expectedRules, rules)

	// Test UpdateRule
	updateParams := UpdateRuleParams{}
	ts.ruleStore.On("UpdateRule", ctx, updateParams).Return(expectedRule, nil)
	rule, err = ts.dbStore.UpdateRule(ctx, updateParams)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test ToggleRuleStatus
	ts.ruleStore.On("ToggleRuleStatus", ctx, ruleID).Return(expectedRule, nil)
	rule, err = ts.dbStore.ToggleRuleStatus(ctx, ruleID)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test VerifyRuleOwnership
	ts.ruleStore.On("VerifyRuleOwnership", ctx, ruleID, userID).Return(nil)
	err = ts.dbStore.VerifyRuleOwnership(ctx, ruleID, userID)
	assert.NoError(t, err)

	// Test DeleteRule
	ts.ruleStore.On("DeleteRule", ctx, ruleID).Return(nil)
	err = ts.dbStore.DeleteRule(ctx, ruleID)
	assert.NoError(t, err)

	ts.ruleStore.AssertExpectations(t)
}

func TestDBStore_LogMethods(t *testing.T) {
	ctx := context.Background()
	ruleID := uuid.New()
	accountID := uuid.New()
	expectedLog := domain.AutomationLog{} // <-- GEWIJZIGD
	ts := newTestStore(t)

	// Test CreateAutomationLog
	logParams := CreateLogParams{}
	ts.logStore.On("CreateAutomationLog", ctx, logParams).Return(nil)
	err := ts.dbStore.CreateAutomationLog(ctx, logParams)
	assert.NoError(t, err)

	// Test HasLogForTrigger
	ts.logStore.On("HasLogForTrigger", ctx, ruleID, "trigger123").Return(true, nil)
	has, err := ts.dbStore.HasLogForTrigger(ctx, ruleID, "trigger123")
	assert.NoError(t, err)
	assert.True(t, has)

	// Test GetLogsForAccount
	expectedLogs := []domain.AutomationLog{expectedLog}
	ts.logStore.On("GetLogsForAccount", ctx, accountID, 50).Return(expectedLogs, nil)
	logs, err := ts.dbStore.GetLogsForAccount(ctx, accountID, 50)
	assert.NoError(t, err)
	assert.Equal(t, expectedLogs, logs)

	ts.logStore.AssertExpectations(t)
}

func TestDBStore_GmailMethods(t *testing.T) {
	ctx := context.Background()
	ruleID := uuid.New()
	accountID := uuid.New()
	expectedRule := domain.GmailAutomationRule{} // <-- GEWIJZIGD
	ts := newTestStore(t)

	// Test CreateGmailAutomationRule
	createParams := CreateGmailAutomationRuleParams{}
	ts.gmailStore.On("CreateGmailAutomationRule", ctx, createParams).Return(expectedRule, nil)
	rule, err := ts.dbStore.CreateGmailAutomationRule(ctx, createParams)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test GetGmailRulesForAccount
	expectedRules := []domain.GmailAutomationRule{expectedRule}
	ts.gmailStore.On("GetGmailRulesForAccount", ctx, accountID).Return(expectedRules, nil)
	rules, err := ts.dbStore.GetGmailRulesForAccount(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, expectedRules, rules)

	// Test UpdateGmailRule
	updateParams := UpdateGmailRuleParams{}
	ts.gmailStore.On("UpdateGmailRule", ctx, updateParams).Return(expectedRule, nil)
	rule, err = ts.dbStore.UpdateGmailRule(ctx, updateParams)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test DeleteGmailRule
	ts.gmailStore.On("DeleteGmailRule", ctx, ruleID).Return(nil)
	err = ts.dbStore.DeleteGmailRule(ctx, ruleID)
	assert.NoError(t, err)

	// Test ToggleGmailRuleStatus
	ts.gmailStore.On("ToggleGmailRuleStatus", ctx, ruleID).Return(expectedRule, nil)
	rule, err = ts.dbStore.ToggleGmailRuleStatus(ctx, ruleID)
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, rule)

	// Test StoreGmailMessage
	msgParams := StoreGmailMessageParams{}
	ts.gmailStore.On("StoreGmailMessage", ctx, msgParams).Return(nil)
	err = ts.dbStore.StoreGmailMessage(ctx, msgParams)
	assert.NoError(t, err)

	// Test StoreGmailThread
	threadParams := StoreGmailThreadParams{}
	ts.gmailStore.On("StoreGmailThread", ctx, threadParams).Return(nil)
	err = ts.dbStore.StoreGmailThread(ctx, threadParams)
	assert.NoError(t, err)

	// Test UpdateGmailMessageStatus
	msgID := "msg123"
	// <-- GEWIJZIGD: Gebruik een aannemelijke constante of string-gebaseerde enum
	status := domain.GmailMessageStatus("read")
	ts.gmailStore.On("UpdateGmailMessageStatus", ctx, accountID, msgID, status).Return(nil)
	err = ts.dbStore.UpdateGmailMessageStatus(ctx, accountID, msgID, status)
	assert.NoError(t, err)

	// Test GetGmailMessagesForAccount
	expectedMsgs := []domain.GmailMessage{} // <-- GEWIJZIGD
	ts.gmailStore.On("GetGmailMessagesForAccount", ctx, accountID, 10).Return(expectedMsgs, nil)
	msgs, err := ts.dbStore.GetGmailMessagesForAccount(ctx, accountID, 10)
	assert.NoError(t, err)
	assert.Equal(t, expectedMsgs, msgs)

	// Test UpdateGmailSyncState
	historyID := "hist123"
	now := time.Now()
	ts.gmailStore.On("UpdateGmailSyncState", ctx, accountID, historyID, now).Return(nil)
	err = ts.dbStore.UpdateGmailSyncState(ctx, accountID, historyID, now)
	assert.NoError(t, err)

	// Test GetGmailSyncState
	ts.gmailStore.On("GetGmailSyncState", ctx, accountID).Return(&historyID, &now, nil)
	histID, syncTime, err := ts.dbStore.GetGmailSyncState(ctx, accountID)
	assert.NoError(t, err)
	assert.Equal(t, &historyID, histID)
	assert.Equal(t, &now, syncTime)

	// Test GetGmailSyncState (Error case)
	mockErr := errors.New("not found")
	ts.gmailStore.On("GetGmailSyncState", ctx, uuid.Nil).Return(nil, nil, mockErr)
	_, _, err = ts.dbStore.GetGmailSyncState(ctx, uuid.Nil)
	assert.Error(t, err)
	assert.Equal(t, mockErr, err)

	ts.gmailStore.AssertExpectations(t)
}
