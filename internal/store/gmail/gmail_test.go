package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- TEST HELPERS ---

var testUUID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
var testAccountID = uuid.MustParse("550e8400-e29b-41d4-a716-446655441111")
var testTime = time.Date(2025, time.November, 16, 12, 0, 0, 0, time.UTC)
var dummyLog = zap.NewNop()
var dummyDesc = stringPtr("Test Description")
var dummyHistoryID = stringPtr("123456789")

func stringPtr(s string) *string {
	return &s
}

// scanRuleHeaders definieert de kolomnamen en de volgorde voor Rules SELECTs
var scanRuleHeaders = []string{
	"id", "connected_account_id", "name", "description", "is_active", "trigger_type",
	"trigger_conditions", "action_type", "action_params", "priority", "created_at", "updated_at",
}

// createMockRuleRow maakt een enkele rij aan voor een GmailAutomationRule (nu dynamisch met params)
func createMockRuleRow(id uuid.UUID, isActive bool, priority int, name string, desc *string, triggerType domain.GmailRuleTriggerType, actionType domain.GmailRuleActionType) *pgxmock.Rows {
	rule := domain.GmailAutomationRule{
		BaseAutomationRule: domain.BaseAutomationRule{
			Name:              name,
			IsActive:          isActive,
			TriggerConditions: json.RawMessage(`{"sender_pattern": "test@mail.com"}`),
			ActionParams:      json.RawMessage(`{"replyMessage": "Auto reply"}`),
			AccountEntity: domain.AccountEntity{
				BaseEntity: domain.BaseEntity{
					ID:        id,
					CreatedAt: testTime,
					UpdatedAt: testTime,
				},
				ConnectedAccountID: testAccountID,
			},
		},
		Description: desc,
		TriggerType: triggerType,
		ActionType:  actionType,
		Priority:    priority,
	}

	return pgxmock.NewRows(scanRuleHeaders).AddRow(
		rule.BaseAutomationRule.ID, rule.BaseAutomationRule.ConnectedAccountID, rule.BaseAutomationRule.Name,
		rule.Description, rule.BaseAutomationRule.IsActive,
		rule.TriggerType, rule.BaseAutomationRule.TriggerConditions, rule.ActionType, rule.BaseAutomationRule.ActionParams,
		rule.Priority, rule.BaseAutomationRule.CreatedAt, rule.BaseAutomationRule.UpdatedAt,
	)
}

// --- CONSTRUCTOR TEST ---

func TestNewGmailStore(t *testing.T) {
	mockDB, _ := pgxmock.NewPool()
	store := NewGmailStore(mockDB, dummyLog)
	assert.NotNil(t, store)
	assert.IsType(t, &GmailStore{}, store)
}

// --- CRUD TESTS (RULES) ---

func TestGmailStore_CreateGmailAutomationRule_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)
	params := CreateGmailAutomationRuleParams{
		ConnectedAccountID: testAccountID,
		Name:               "New Rule",
		Description:        dummyDesc,
		IsActive:           true,
		TriggerType:        domain.GmailTriggerStarred,
		TriggerConditions:  json.RawMessage(`{}`),
		ActionType:         domain.GmailActionStar,
		ActionParams:       json.RawMessage(`{}`),
		Priority:           1,
	}

	mockDB.ExpectQuery(`INSERT INTO gmail_automation_rules`).
		WithArgs(params.ConnectedAccountID, params.Name, params.Description, params.IsActive,
			params.TriggerType, params.TriggerConditions, params.ActionType, params.ActionParams, params.Priority).
		WillReturnRows(createMockRuleRow(testUUID, true, 1, params.Name, params.Description, params.TriggerType, params.ActionType))

	rule, err := store.CreateGmailAutomationRule(context.Background(), params)
	assert.NoError(t, err)
	assert.Equal(t, testUUID, rule.ID)
	assert.Equal(t, "New Rule", rule.Name)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_CreateGmailAutomationRule_DBError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)
	params := CreateGmailAutomationRuleParams{
		ConnectedAccountID: testAccountID,
		Name:               "Fail",
		Description:        dummyDesc,
		IsActive:           true,
		TriggerType:        domain.GmailTriggerStarred,
		TriggerConditions:  json.RawMessage(`{}`),
		ActionType:         domain.GmailActionStar,
		ActionParams:       json.RawMessage(`{}`),
		Priority:           1,
	}

	mockDB.ExpectQuery(`INSERT INTO gmail_automation_rules`).
		WithArgs(params.ConnectedAccountID, params.Name, params.Description, params.IsActive,
			params.TriggerType, params.TriggerConditions, params.ActionType, params.ActionParams, params.Priority).
		WillReturnError(fmt.Errorf("database insert error"))

	_, err = store.CreateGmailAutomationRule(context.Background(), params)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database insert error")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_GetGmailRulesForAccount_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	rows := pgxmock.NewRows(scanRuleHeaders).
		AddRow(testUUID, testAccountID, "Rule 1", dummyDesc, true, domain.GmailTriggerNewMessage, json.RawMessage(`{}`), domain.GmailActionArchive, json.RawMessage(`{}`), 10, testTime, testTime).
		AddRow(uuid.New(), testAccountID, "Rule 2", dummyDesc, false, domain.GmailTriggerStarred, json.RawMessage(`{}`), domain.GmailActionStar, json.RawMessage(`{}`), 5, testTime, testTime)

	mockDB.ExpectQuery(`SELECT .* FROM gmail_automation_rules WHERE connected_account_id = \$1`).
		WithArgs(testAccountID).
		WillReturnRows(rows)

	rules, err := store.GetGmailRulesForAccount(context.Background(), testAccountID)
	assert.NoError(t, err)
	assert.Len(t, rules, 2)
	assert.Equal(t, "Rule 1", rules[0].Name)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_GetGmailRulesForAccount_NoRows(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectQuery(`SELECT .* FROM gmail_automation_rules WHERE connected_account_id = \$1`).
		WithArgs(testAccountID).
		WillReturnRows(pgxmock.NewRows(scanRuleHeaders)) // Lege set

	rules, err := store.GetGmailRulesForAccount(context.Background(), testAccountID)
	assert.NoError(t, err)
	assert.Empty(t, rules)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_GetGmailRulesForAccount_ScanError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	rows := pgxmock.NewRows(scanRuleHeaders).
		AddRow(testUUID, testAccountID, "Rule 1", dummyDesc, "NOT_BOOL", domain.GmailTriggerNewMessage, json.RawMessage(`{}`), domain.GmailActionArchive, json.RawMessage(`{}`), 10, testTime, testTime) // IsActive is verkeerd

	mockDB.ExpectQuery(`SELECT .* FROM gmail_automation_rules WHERE connected_account_id = \$1`).
		WithArgs(testAccountID).
		WillReturnRows(rows)

	_, err = store.GetGmailRulesForAccount(context.Background(), testAccountID)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "Destination kind 'bool' not supported for value kind 'string' of column 'is_active'")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_UpdateGmailRule_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)
	newDesc := stringPtr("Updated Description")
	params := UpdateGmailRuleParams{
		RuleID:            testUUID,
		Name:              "Updated Name",
		Description:       newDesc,
		TriggerType:       domain.GmailTriggerSubjectMatch,
		TriggerConditions: json.RawMessage(`{"subject_pattern": "new"}`),
		ActionType:        domain.GmailActionForward,
		ActionParams:      json.RawMessage(`{"email": "forward@mail.com"}`),
		Priority:          5,
	}

	expectedRule := createMockRuleRow(testUUID, true, params.Priority, params.Name, params.Description, params.TriggerType, params.ActionType)
	// Mock ExpectExec omdat we nu de RETURNING gebruiken (QueryRow)
	mockDB.ExpectQuery(`UPDATE gmail_automation_rules`).
		WithArgs(params.Name, params.Description, params.TriggerType, params.TriggerConditions,
			params.ActionType, params.ActionParams, params.Priority, params.RuleID).
		WillReturnRows(expectedRule)

	rule, err := store.UpdateGmailRule(context.Background(), params)
	assert.NoError(t, err)
	assert.Equal(t, testUUID, rule.ID)
	assert.Equal(t, "Updated Name", rule.Name)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_UpdateGmailRule_NotFound(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)
	params := UpdateGmailRuleParams{
		RuleID:            testUUID,
		Name:              "Updated Name",
		Description:       dummyDesc,
		TriggerType:       domain.GmailTriggerSubjectMatch,
		TriggerConditions: json.RawMessage(`{"subject_pattern": "new"}`),
		ActionType:        domain.GmailActionForward,
		ActionParams:      json.RawMessage(`{"email": "forward@mail.com"}`),
		Priority:          5,
	}

	mockDB.ExpectQuery(`UPDATE gmail_automation_rules`).
		WithArgs(params.Name, params.Description, params.TriggerType, params.TriggerConditions,
			params.ActionType, params.ActionParams, params.Priority, params.RuleID).
		WillReturnError(pgx.ErrNoRows)

	_, err = store.UpdateGmailRule(context.Background(), params)
	assert.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_DeleteGmailRule_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectExec(`DELETE FROM gmail_automation_rules WHERE id = \$1`).
		WithArgs(testUUID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = store.DeleteGmailRule(context.Background(), testUUID)
	assert.NoError(t, err)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_DeleteGmailRule_NotFound(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectExec(`DELETE FROM gmail_automation_rules WHERE id = \$1`).
		WithArgs(testUUID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = store.DeleteGmailRule(context.Background(), testUUID)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "no Gmail rule found with ID")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_DeleteGmailRule_DBError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectExec(`DELETE FROM gmail_automation_rules WHERE id = \$1`).
		WithArgs(testUUID).
		WillReturnError(fmt.Errorf("database delete error"))

	err = store.DeleteGmailRule(context.Background(), testUUID)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database delete error")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_ToggleGmailRuleStatus_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	expectedRule := createMockRuleRow(testUUID, false, 1, "Test Rule", dummyDesc, domain.GmailTriggerNewMessage, domain.GmailActionArchive)

	mockDB.ExpectQuery(`UPDATE gmail_automation_rules SET is_active = NOT is_active`).
		WithArgs(testUUID).
		WillReturnRows(expectedRule)

	rule, err := store.ToggleGmailRuleStatus(context.Background(), testUUID)
	assert.NoError(t, err)
	assert.Equal(t, testUUID, rule.ID)
	assert.False(t, rule.IsActive) // Controleer de status toggle
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_ToggleGmailRuleStatus_NotFound(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectQuery(`UPDATE gmail_automation_rules SET is_active = NOT is_active`).
		WithArgs(testUUID).
		WillReturnError(pgx.ErrNoRows)

	_, err = store.ToggleGmailRuleStatus(context.Background(), testUUID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

// --- SYNC STATE TESTS ---

func TestGmailStore_UpdateGmailSyncState_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectExec(`UPDATE connected_accounts SET gmail_history_id = \$1, gmail_last_sync = \$2`).
		WithArgs(*dummyHistoryID, testTime, testAccountID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = store.UpdateGmailSyncState(context.Background(), testAccountID, *dummyHistoryID, testTime)
	assert.NoError(t, err)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_UpdateGmailSyncState_DBError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectExec(`UPDATE connected_accounts SET gmail_history_id = \$1, gmail_last_sync = \$2`).
		WithArgs(*dummyHistoryID, testTime, testAccountID).
		WillReturnError(fmt.Errorf("database update error"))

	err = store.UpdateGmailSyncState(context.Background(), testAccountID, *dummyHistoryID, testTime)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "database update error")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_GetGmailSyncState_Success(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	rows := pgxmock.NewRows([]string{"gmail_history_id", "gmail_last_sync"}).
		AddRow(*dummyHistoryID, testTime)

	mockDB.ExpectQuery(`SELECT gmail_history_id, gmail_last_sync FROM connected_accounts WHERE id = \$1`).
		WithArgs(testAccountID).
		WillReturnRows(rows)

	historyID, lastSync, err := store.GetGmailSyncState(context.Background(), testAccountID)
	assert.NoError(t, err)
	assert.Equal(t, *dummyHistoryID, *historyID)
	assert.Equal(t, testTime, *lastSync)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_GetGmailSyncState_NotFound(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectQuery(`SELECT gmail_history_id, gmail_last_sync FROM connected_accounts WHERE id = \$1`).
		WithArgs(testAccountID).
		WillReturnError(pgx.ErrNoRows)

	historyID, lastSync, err := store.GetGmailSyncState(context.Background(), testAccountID)
	assert.Error(t, err)
	assert.Nil(t, historyID)
	assert.Nil(t, lastSync)
	assert.ErrorContains(t, err, "account not found")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGmailStore_GetGmailSyncState_DBError(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	store := NewGmailStore(mockDB, dummyLog)

	mockDB.ExpectQuery(`SELECT gmail_history_id, gmail_last_sync FROM connected_accounts WHERE id = \$1`).
		WithArgs(testAccountID).
		WillReturnError(fmt.Errorf("db connection failed"))

	historyID, lastSync, err := store.GetGmailSyncState(context.Background(), testAccountID)
	assert.Error(t, err)
	assert.Nil(t, historyID)
	assert.Nil(t, lastSync)
	assert.ErrorContains(t, err, "db connection failed")
	assert.NoError(t, mockDB.ExpectationsWereMet())
}
