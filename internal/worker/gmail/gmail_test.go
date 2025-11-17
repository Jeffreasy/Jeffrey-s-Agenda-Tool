//go:build integration

package gmail

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

// --- MOCKS ---

type MockStore struct {
	mock.Mock
}

func (m *MockStore) GetGmailSyncState(ctx context.Context, accountID uuid.UUID) (*string, *time.Time, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(*string), args.Get(1).(*time.Time), args.Error(2)
}

func (m *MockStore) GetGmailRulesForAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GmailAutomationRule, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).([]domain.GmailAutomationRule), args.Error(1)
}

func (m *MockStore) UpdateGmailSyncState(ctx context.Context, accountID uuid.UUID, historyID string, lastSync time.Time) error {
	args := m.Called(ctx, accountID, historyID, lastSync)
	return args.Error(0)
}

func (m *MockStore) StoreGmailMessage(ctx context.Context, arg store.StoreGmailMessageParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

// Overige methodes indien nodig
func (m *MockStore) GetValidTokenForAccount(ctx context.Context, accountID uuid.UUID) (*oauth2.Token, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

// MockGmailService mocks the Gmail service
type MockGmailService struct {
	mock.Mock
}

func (m *MockGmailService) UsersMessagesList(userID string) *gmail.UsersMessagesListCall {
	args := m.Called(userID)
	return args.Get(0).(*gmail.UsersMessagesListCall)
}

func (m *MockGmailService) UsersMessagesGet(userID, msgID string) *gmail.UsersMessagesGetCall {
	args := m.Called(userID, msgID)
	return args.Get(0).(*gmail.UsersMessagesGetCall)
}

func (m *MockGmailService) UsersHistoryList(userID string) *gmail.UsersHistoryListCall {
	args := m.Called(userID)
	return args.Get(0).(*gmail.UsersHistoryListCall)
}

func (m *MockGmailService) UsersGetProfile(userID string) *gmail.UsersGetProfileCall {
	args := m.Called(userID)
	return args.Get(0).(*gmail.UsersGetProfileCall)
}

// Mock calls for chained methods
type MockUsersMessagesListCall struct {
	mock.Mock
}

func (m *MockUsersMessagesListCall) MaxResults(max int64) *gmail.UsersMessagesListCall {
	m.Called(max)
	return m
}

func (m *MockUsersMessagesListCall) Q(q string) *gmail.UsersMessagesListCall {
	m.Called(q)
	return m
}

func (m *MockUsersMessagesListCall) LabelIds(labelIds ...string) *gmail.UsersMessagesListCall {
	m.Called(labelIds)
	return m
}

func (m *MockUsersMessagesListCall) Do() (*gmail.ListMessagesResponse, error) {
	args := m.Called()
	return args.Get(0).(*gmail.ListMessagesResponse), args.Error(1)
}

type MockUsersMessagesGetCall struct {
	mock.Mock
}

func (m *MockUsersMessagesGetCall) Format(format string) *gmail.UsersMessagesGetCall {
	m.Called(format)
	return m
}

func (m *MockUsersMessagesGetCall) Do() (*gmail.Message, error) {
	args := m.Called()
	return args.Get(0).(*gmail.Message), args.Error(1)
}

type MockUsersHistoryListCall struct {
	mock.Mock
}

func (m *MockUsersHistoryListCall) StartHistoryId(start uint64) *gmail.UsersHistoryListCall {
	m.Called(start)
	return m
}

func (m *MockUsersHistoryListCall) Do() (*gmail.ListHistoryResponse, error) {
	args := m.Called()
	return args.Get(0).(*gmail.ListHistoryResponse), args.Error(1)
}

type MockUsersGetProfileCall struct {
	mock.Mock
}

func (m *MockUsersGetProfileCall) Do() (*gmail.Profile, error) {
	args := m.Called()
	return args.Get(0).(*gmail.Profile), args.Error(1)
}

// --- TEST CASES ---

func TestNewGmailProcessor(t *testing.T) {
	mockStore := new(MockStore)
	p := NewGmailProcessor(mockStore)
	assert.NotNil(t, p)
	assert.NotNil(t, p.newService)
}

func TestProcessMessages_ServiceCreationFails(t *testing.T) {
	mockStore := new(MockStore)
	p := NewGmailProcessor(mockStore)
	p.newService = func(ctx context.Context, client *http.Client) (*gmail.Service, error) {
		return nil, errors.New("service creation failed")
	}

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not create Gmail service")
}

func TestProcessMessages_NoActiveRules(t *testing.T) {
	mockStore := new(MockStore)
	p := NewGmailProcessor(mockStore)

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	mockStore.On("GetGmailSyncState", mock.Anything, acc.ID).Return((*string)(nil), (*time.Time)(nil), nil)
	mockStore.On("GetGmailRulesForAccount", mock.Anything, acc.ID).Return([]domain.GmailAutomationRule{}, nil)

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestProcessMessages_FetchRulesError(t *testing.T) {
	mockStore := new(MockStore)
	p := NewGmailProcessor(mockStore)

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	mockStore.On("GetGmailSyncState", mock.Anything, acc.ID).Return((*string)(nil), (*time.Time)(nil), nil)
	mockStore.On("GetGmailRulesForAccount", mock.Anything, acc.ID).Return(nil, errors.New("rules fetch error"))

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not fetch Gmail rules")
	mockStore.AssertExpectations(t)
}

func TestProcessMessages_FullSync(t *testing.T) {
	mockStore := new(MockStore)
	mockSrv := new(MockGmailService)
	p := NewGmailProcessor(mockStore)
	p.newService = func(ctx context.Context, client *http.Client) (*gmail.Service, error) {
		return &gmail.Service{}, nil // Dummy service
	}

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	// Mock sync state (nil, full sync)
	mockStore.On("GetGmailSyncState", mock.Anything, acc.ID).Return((*string)(nil), (*time.Time)(nil), nil)

	// Mock rules (1 active rule)
	rules := []domain.GmailAutomationRule{{IsActive: true}}
	mockStore.On("GetGmailRulesForAccount", mock.Anything, acc.ID).Return(rules, nil)

	// Mock fetchRecentMessages (return 1 message)
	mockMsg := &gmail.Message{Id: "msg1"}
	p.fetchRecentMessages = func(srv *gmail.Service, acc *domain.ConnectedAccount) ([]*gmail.Message, error) {
		return []*gmail.Message{mockMsg}, nil
	}

	// Mock processMessageAgainstRules (success)
	p.processMessageAgainstRules = func(ctx context.Context, srv *gmail.Service, acc *domain.ConnectedAccount, msg *gmail.Message, rules []domain.GmailAutomationRule) error {
		return nil
	}

	// Mock profile for initial history ID
	mockProfileCall := new(MockUsersGetProfileCall)
	mockSrv.On("UsersGetProfile", "me").Return(mockProfileCall)
	mockProfileCall.On("Do").Return(&gmail.Profile{HistoryId: 12345}, nil)

	// Mock update sync state
	mockStore.On("UpdateGmailSyncState", mock.Anything, acc.ID, "12345", mock.AnythingOfType("time.Time")).Return(nil)

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockSrv.AssertExpectations(t)
	mockProfileCall.AssertExpectations(t)
}

func TestProcessMessages_IncrementalSync(t *testing.T) {
	mockStore := new(MockStore)
	mockSrv := new(MockGmailService)
	p := NewGmailProcessor(mockStore)
	p.newService = func(ctx context.Context, client *http.Client) (*gmail.Service, error) {
		return &gmail.Service{}, nil
	}

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	historyIDStr := "1000"
	historyID := &historyIDStr
	lastSync := time.Now().Add(-1 * time.Hour)
	lastSyncPtr := &lastSync

	// Mock sync state
	mockStore.On("GetGmailSyncState", mock.Anything, acc.ID).Return(historyID, lastSyncPtr, nil)

	// Mock rules
	rules := []domain.GmailAutomationRule{{IsActive: true}}
	mockStore.On("GetGmailRulesForAccount", mock.Anything, acc.ID).Return(rules, nil)

	// Mock history list
	mockHistoryCall := new(MockUsersHistoryListCall)
	mockSrv.On("UsersHistoryList", "me").Return(mockHistoryCall)
	mockHistoryCall.On("StartHistoryId", uint64(1000)).Return(mockHistoryCall)
	mockHistoryCall.On("Do").Return(&gmail.ListHistoryResponse{HistoryId: 2000}, nil)

	// Mock processHistoryItems (return 1 message)
	mockMsg := &gmail.Message{Id: "msg1"}
	p.processHistoryItems = func(history *gmail.ListHistoryResponse, srv *gmail.Service) []*gmail.Message {
		return []*gmail.Message{mockMsg}
	}

	// Mock processMessageAgainstRules
	p.processMessageAgainstRules = func(ctx context.Context, srv *gmail.Service, acc *domain.ConnectedAccount, msg *gmail.Message, rules []domain.GmailAutomationRule) error {
		return nil
	}

	// Mock update sync state
	mockStore.On("UpdateGmailSyncState", mock.Anything, acc.ID, "2000", mock.AnythingOfType("time.Time")).Return(nil)

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockSrv.AssertExpectations(t)
	mockHistoryCall.AssertExpectations(t)
}

func TestProcessMessages_HistoryAPIFailsFallback(t *testing.T) {
	mockStore := new(MockStore)
	mockSrv := new(MockGmailService)
	p := NewGmailProcessor(mockStore)
	p.newService = func(ctx context.Context, client *http.Client) (*gmail.Service, error) {
		return &gmail.Service{}, nil
	}

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	historyIDStr := "1000"
	historyID := &historyIDStr
	lastSync := time.Now().Add(-1 * time.Hour)
	lastSyncPtr := &lastSync

	mockStore.On("GetGmailSyncState", mock.Anything, acc.ID).Return(historyID, lastSyncPtr, nil)

	rules := []domain.GmailAutomationRule{{IsActive: true}}
	mockStore.On("GetGmailRulesForAccount", mock.Anything, acc.ID).Return(rules, nil)

	// Mock history failure
	mockHistoryCall := new(MockUsersHistoryListCall)
	mockSrv.On("UsersHistoryList", "me").Return(mockHistoryCall)
	mockHistoryCall.On("StartHistoryId", uint64(1000)).Return(mockHistoryCall)
	mockHistoryCall.On("Do").Return((*gmail.ListHistoryResponse)(nil), errors.New("history API error"))

	// Mock fallback to fetchRecentMessages
	mockMsg := &gmail.Message{Id: "msg1"}
	p.fetchRecentMessages = func(srv *gmail.Service, acc *domain.ConnectedAccount) ([]*gmail.Message, error) {
		return []*gmail.Message{mockMsg}, nil
	}

	p.processMessageAgainstRules = func(ctx context.Context, srv *gmail.Service, acc *domain.ConnectedAccount, msg *gmail.Message, rules []domain.GmailAutomationRule) error {
		return nil
	}

	// No update sync state since fallback

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockSrv.AssertExpectations(t)
	mockHistoryCall.AssertExpectations(t)
}

func TestProcessMessages_InvalidHistoryIDFallback(t *testing.T) {
	mockStore := new(MockStore)
	mockSrv := new(MockGmailService)
	p := NewGmailProcessor(mockStore)
	p.newService = func(ctx context.Context, client *http.Client) (*gmail.Service, error) {
		return &gmail.Service{}, nil
	}

	acc := &domain.ConnectedAccount{ID: uuid.New()}
	token := &oauth2.Token{}

	historyIDStr := "invalid"
	historyID := &historyIDStr
	lastSync := time.Now().Add(-1 * time.Hour)
	lastSyncPtr := &lastSync

	mockStore.On("GetGmailSyncState", mock.Anything, acc.ID).Return(historyID, lastSyncPtr, nil)

	rules := []domain.GmailAutomationRule{{IsActive: true}}
	mockStore.On("GetGmailRulesForAccount", mock.Anything, acc.ID).Return(rules, nil)

	// No history call since invalid ID

	mockMsg := &gmail.Message{Id: "msg1"}
	p.fetchRecentMessages = func(srv *gmail.Service, acc *domain.ConnectedAccount) ([]*gmail.Message, error) {
		return []*gmail.Message{mockMsg}, nil
	}

	p.processMessageAgainstRules = func(ctx context.Context, srv *gmail.Service, acc *domain.ConnectedAccount, msg *gmail.Message, rules []domain.GmailAutomationRule) error {
		return nil
	}

	mockProfileCall := new(MockUsersGetProfileCall)
	mockSrv.On("UsersGetProfile", "me").Return(mockProfileCall)
	mockProfileCall.On("Do").Return(&gmail.Profile{HistoryId: 12345}, nil)

	mockStore.On("UpdateGmailSyncState", mock.Anything, acc.ID, "12345", mock.AnythingOfType("time.Time")).Return(nil)

	err := p.ProcessMessages(context.Background(), acc, token)
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	mockSrv.AssertExpectations(t)
	mockProfileCall.AssertExpectations(t)
}

// --- INTEGRATION TESTS (met testcontainers) ---

func TestGmailProcessor_Integration(t *testing.T) {
	ctx := context.Background()
	// Setup testcontainers PostgreSQL (from TESTING.md example)
	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:15-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_USER":     "testuser",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	// Get connection string
	connStr, err := pgContainer.Endpoint(ctx, "postgres")
	require.NoError(t, err)

	// Connect to DB
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	// Run migrations (assume RunMigrations function exists)
	err = RunMigrations(pool)
	require.NoError(t, err)

	// Create processor with real store
	realStore := NewStore(pool) // Assume NewStore from store.go
	p := NewGmailProcessor(realStore)

	// Create test account
	accParams := UpsertConnectedAccountParams{ /* fill params */ }
	acc, err := realStore.UpsertConnectedAccount(ctx, accParams)
	require.NoError(t, err)

	token := &oauth2.Token{ /* dummy token */ }

	// Test full process (add assertions based on DB state after)
	err = p.ProcessMessages(ctx, &acc, token)
	assert.NoError(t, err)

	// Verify DB state (e.g., logs inserted)
	logs, err := realStore.GetLogsForAccount(ctx, acc.ID, 10)
	assert.NoError(t, err)
	assert.Len(t, logs, 0) // Adjust based on expected
}
