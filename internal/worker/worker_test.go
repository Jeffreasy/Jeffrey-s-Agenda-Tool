package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// createValidToken creates a valid oauth2.Token for testing
func createValidToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: "valid_access_token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
}

func TestNewWorker(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)
	assert.NotNil(t, worker)
	assert.Equal(t, mockStore, worker.store)
	assert.Equal(t, testLogger, worker.logger)
	assert.NotNil(t, worker.calendarProcessor)
	assert.NotNil(t, worker.gmailProcessor)
}

func TestNewWorker_NilStore(t *testing.T) {
	testLogger := zap.NewNop()

	worker, err := NewWorker(nil, testLogger)
	assert.NoError(t, err)
	assert.NotNil(t, worker)
	assert.Nil(t, worker.store)
	assert.Equal(t, testLogger, worker.logger)
	assert.NotNil(t, worker.calendarProcessor)
	assert.NotNil(t, worker.gmailProcessor)
}

func TestWorker_Start(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	// Mock GetActiveAccounts to return empty slice to avoid processing
	mockStore.On("GetActiveAccounts", mock.Anything).Return([]domain.ConnectedAccount{}, nil).Maybe()

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	// Start the worker
	worker.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// We can't easily test the goroutine directly, but we can verify the worker was created
	assert.NotNil(t, worker)
	mockStore.AssertExpectations(t)
}

func TestWorker_doWork(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	// Mock GetActiveAccounts to return empty slice
	mockStore.On("GetActiveAccounts", mock.Anything).Return([]domain.ConnectedAccount{}, nil)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	// Call doWork directly
	worker.doWork()

	// Verify the mock was called
	mockStore.AssertExpectations(t)
}

func TestWorker_doWork_GetActiveAccountsError(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	expectedErr := errors.New("database error")
	mockStore.On("GetActiveAccounts", mock.Anything).Return([]domain.ConnectedAccount{}, expectedErr)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	// Call doWork directly - should not panic
	worker.doWork()

	// Verify the mock was called
	mockStore.AssertExpectations(t)
}

func TestWorker_checkAccounts(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	accounts := []domain.ConnectedAccount{
		{
			ID:               uuid.New(),
			UserID:           uuid.New(),
			Provider:         domain.ProviderGoogle,
			Email:            "test@example.com",
			ProviderUserID:   "provider123",
			AccessToken:      []byte("token"),
			RefreshToken:     []byte("refresh"),
			TokenExpiry:      time.Now().Add(time.Hour),
			Scopes:           []string{"scope1"},
			Status:           domain.StatusActive,
			GmailSyncEnabled: false, // Disable Gmail to avoid Gmail processor calls
		},
	}

	mockStore.On("GetActiveAccounts", mock.Anything).Return(accounts, nil)
	mockStore.On("GetValidTokenForAccount", mock.Anything, accounts[0].ID).Return(createValidToken(), nil)
	mockStore.On("GetRulesForAccount", mock.Anything, accounts[0].ID).Return([]domain.AutomationRule{}, nil) // No rules to avoid processing
	mockStore.On("UpdateAccountLastChecked", mock.Anything, accounts[0].ID).Return(nil)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	// Mock the processors to avoid actual processing
	// We can't easily mock the processors since they're not interfaces, but we can test the logic

	ctx := context.Background()
	err = worker.checkAccounts(ctx)

	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestWorker_checkAccounts_GetActiveAccountsError(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	expectedErr := errors.New("database error")
	mockStore.On("GetActiveAccounts", mock.Anything).Return([]domain.ConnectedAccount{}, expectedErr)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	ctx := context.Background()
	err = worker.checkAccounts(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not get active accounts")
	mockStore.AssertExpectations(t)
}

func TestWorker_processAccount_ValidToken(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	account := &domain.ConnectedAccount{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         domain.ProviderGoogle,
		Email:            "test@example.com",
		ProviderUserID:   "provider123",
		AccessToken:      []byte("token"),
		RefreshToken:     []byte("refresh"),
		TokenExpiry:      time.Now().Add(time.Hour),
		Scopes:           []string{"scope1"},
		Status:           domain.StatusActive,
		GmailSyncEnabled: false, // Disable Gmail to avoid Gmail processor calls
	}

	mockStore.On("GetValidTokenForAccount", mock.Anything, account.ID).Return(createValidToken(), nil)
	mockStore.On("GetRulesForAccount", mock.Anything, account.ID).Return([]domain.AutomationRule{}, nil) // No rules to avoid processing
	mockStore.On("UpdateAccountLastChecked", mock.Anything, account.ID).Return(nil)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	ctx := context.Background()
	// This will call the processors, but since we can't mock them easily,
	// we'll just ensure no panic occurs
	worker.processAccount(ctx, account)

	mockStore.AssertExpectations(t)
}

func TestWorker_processAccount_TokenRevoked(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	account := &domain.ConnectedAccount{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         domain.ProviderGoogle,
		Email:            "test@example.com",
		ProviderUserID:   "provider123",
		AccessToken:      []byte("token"),
		RefreshToken:     []byte("refresh"),
		TokenExpiry:      time.Now().Add(time.Hour),
		Scopes:           []string{"scope1"},
		Status:           domain.StatusActive,
		GmailSyncEnabled: false, // Disable Gmail
	}

	mockStore.On("GetValidTokenForAccount", mock.Anything, account.ID).Return(nil, store.ErrTokenRevoked)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	ctx := context.Background()
	// Should not panic and should not call UpdateAccountLastChecked
	worker.processAccount(ctx, account)

	mockStore.AssertExpectations(t)
	// Verify UpdateAccountLastChecked was NOT called
	mockStore.AssertNotCalled(t, "UpdateAccountLastChecked", mock.Anything, account.ID)
}

func TestWorker_processAccount_TokenError(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	account := &domain.ConnectedAccount{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         domain.ProviderGoogle,
		Email:            "test@example.com",
		ProviderUserID:   "provider123",
		AccessToken:      []byte("token"),
		RefreshToken:     []byte("refresh"),
		TokenExpiry:      time.Now().Add(time.Hour),
		Scopes:           []string{"scope1"},
		Status:           domain.StatusActive,
		GmailSyncEnabled: false, // Disable Gmail
	}

	expectedErr := errors.New("token refresh failed")
	mockStore.On("GetValidTokenForAccount", mock.Anything, account.ID).Return(nil, expectedErr)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	ctx := context.Background()
	// Should not panic and should not call UpdateAccountLastChecked
	worker.processAccount(ctx, account)

	mockStore.AssertExpectations(t)
	// Verify UpdateAccountLastChecked was NOT called
	mockStore.AssertNotCalled(t, "UpdateAccountLastChecked", mock.Anything, account.ID)
}

func TestWorker_processAccount_UpdateLastCheckedError(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	account := &domain.ConnectedAccount{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         domain.ProviderGoogle,
		Email:            "test@example.com",
		ProviderUserID:   "provider123",
		AccessToken:      []byte("token"),
		RefreshToken:     []byte("refresh"),
		TokenExpiry:      time.Now().Add(time.Hour),
		Scopes:           []string{"scope1"},
		Status:           domain.StatusActive,
		GmailSyncEnabled: false, // Disable Gmail to avoid processor calls
	}

	mockStore.On("GetValidTokenForAccount", mock.Anything, account.ID).Return(createValidToken(), nil)
	mockStore.On("GetRulesForAccount", mock.Anything, account.ID).Return([]domain.AutomationRule{}, nil) // No rules to avoid processing
	expectedErr := errors.New("update failed")
	mockStore.On("UpdateAccountLastChecked", mock.Anything, account.ID).Return(expectedErr)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	ctx := context.Background()
	// Should not panic despite the error
	worker.processAccount(ctx, account)

	mockStore.AssertExpectations(t)
}

func TestWorker_processAccount_GmailSyncDisabled(t *testing.T) {
	testLogger := zap.NewNop()
	mockStore := &store.MockStore{}

	account := &domain.ConnectedAccount{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         domain.ProviderGoogle,
		Email:            "test@example.com",
		ProviderUserID:   "provider123",
		AccessToken:      []byte("token"),
		RefreshToken:     []byte("refresh"),
		TokenExpiry:      time.Now().Add(time.Hour),
		Scopes:           []string{"scope1"},
		Status:           domain.StatusActive,
		GmailSyncEnabled: false, // Gmail sync disabled
	}

	mockStore.On("GetValidTokenForAccount", mock.Anything, account.ID).Return(createValidToken(), nil)
	mockStore.On("GetRulesForAccount", mock.Anything, account.ID).Return([]domain.AutomationRule{}, nil) // No rules to avoid processing
	mockStore.On("UpdateAccountLastChecked", mock.Anything, account.ID).Return(nil)

	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)

	ctx := context.Background()
	worker.processAccount(ctx, account)

	mockStore.AssertExpectations(t)
}
