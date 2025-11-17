package rule

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRuleStore is een helper die een RuleStore en een mock pool aanmaakt.
func setupRuleStore(t *testing.T) (RuleStorer, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)

	// We kunnen de mockPool doorgeven aan de (nu refactored) NewRuleStore
	store := NewRuleStore(mockPool)
	return store, mockPool
}

// Definitie van de kolommen die door de queries worden geretourneerd
var ruleColumns = []string{
	"id", "connected_account_id", "name", "is_active",
	"trigger_conditions", "action_params", "created_at", "updated_at",
}

// Helper om een standaard mock-regel te maken
func mockRuleData(ruleID, accountID uuid.UUID, name string, active bool) (uuid.UUID, uuid.UUID, string, bool, json.RawMessage, json.RawMessage, time.Time, time.Time) {
	return ruleID, accountID, name, active,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		time.Now(), time.Now()
}

func TestRuleStore_CreateAutomationRule(t *testing.T) {
	store, mockPool := setupRuleStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	accountID := uuid.New()
	ruleID := uuid.New()

	params := CreateAutomationRuleParams{
		ConnectedAccountID: accountID,
		Name:               "Test Rule",
		TriggerConditions:  json.RawMessage(`{"key":"value"}`),
		ActionParams:       json.RawMessage(`{}`),
	}

	// Mock de data die de DB teruggeeft
	rows := pgxmock.NewRows(ruleColumns).AddRow(
		ruleID, params.ConnectedAccountID, params.Name, true, // is_active default op true
		params.TriggerConditions, params.ActionParams, time.Now(), time.Now(),
	)

	mockPool.ExpectQuery("^INSERT INTO automation_rules").
		WithArgs(
			params.ConnectedAccountID, params.Name,
			params.TriggerConditions, params.ActionParams,
		).
		WillReturnRows(rows)

	// Act
	rule, err := store.CreateAutomationRule(ctx, params)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, ruleID, rule.ID)
	assert.Equal(t, "Test Rule", rule.Name)
	assert.True(t, rule.IsActive)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRuleStore_GetRuleByID(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		store, mockPool := setupRuleStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()
		accountID := uuid.New()

		rows := pgxmock.NewRows(ruleColumns).AddRow(
			mockRuleData(ruleID, accountID, "Found Rule", true),
		)

		mockPool.ExpectQuery("^SELECT id, connected_account_id").
			WithArgs(ruleID).
			WillReturnRows(rows)

		// Act
		rule, err := store.GetRuleByID(ctx, ruleID)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, ruleID, rule.ID)
		assert.Equal(t, "Found Rule", rule.Name)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("Not Found", func(t *testing.T) {
		store, mockPool := setupRuleStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()

		mockPool.ExpectQuery("^SELECT id, connected_account_id").
			WithArgs(ruleID).
			WillReturnError(pgx.ErrNoRows) // Simuleer "not found"

		// Act
		_, err := store.GetRuleByID(ctx, ruleID)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rule not found") // Check de custom error
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestRuleStore_GetRulesForAccount(t *testing.T) {
	store, mockPool := setupRuleStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	accountID := uuid.New()

	rows := pgxmock.NewRows(ruleColumns).
		AddRow(mockRuleData(uuid.New(), accountID, "Rule 1", true)).
		AddRow(mockRuleData(uuid.New(), accountID, "Rule 2", false))

	mockPool.ExpectQuery("^SELECT id, connected_account_id").
		WithArgs(accountID).
		WillReturnRows(rows)

	// Act
	rules, err := store.GetRulesForAccount(ctx, accountID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, rules, 2)
	assert.Equal(t, "Rule 1", rules[0].Name)
	assert.Equal(t, "Rule 2", rules[1].Name)
	assert.False(t, rules[1].IsActive)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRuleStore_ToggleRuleStatus(t *testing.T) {
	store, mockPool := setupRuleStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	ruleID := uuid.New()
	accountID := uuid.New()

	// De teruggestuurde regel is nu 'false' (getoggled)
	rows := pgxmock.NewRows(ruleColumns).AddRow(
		mockRuleData(ruleID, accountID, "Toggled Rule", false),
	)

	mockPool.ExpectQuery("^UPDATE automation_rules SET is_active = NOT is_active").
		WithArgs(ruleID).
		WillReturnRows(rows)

	// Act
	rule, err := store.ToggleRuleStatus(ctx, ruleID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, ruleID, rule.ID)
	assert.False(t, rule.IsActive) // Controleer of de status is gewijzigd
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestRuleStore_VerifyRuleOwnership(t *testing.T) {
	t.Run("Success (Owner)", func(t *testing.T) {
		store, mockPool := setupRuleStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()
		userID := uuid.New()

		// De query retourneert '1'
		rows := pgxmock.NewRows([]string{"1"}).AddRow(1)

		mockPool.ExpectQuery("^SELECT 1").
			WithArgs(ruleID, userID).
			WillReturnRows(rows)

		// Act
		err := store.VerifyRuleOwnership(ctx, ruleID, userID)

		// Assert
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("Failure (Not Owner)", func(t *testing.T) {
		store, mockPool := setupRuleStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()
		userID := uuid.New()

		// De query retourneert geen rijen
		mockPool.ExpectQuery("^SELECT 1").
			WithArgs(ruleID, userID).
			WillReturnError(pgx.ErrNoRows)

		// Act
		err := store.VerifyRuleOwnership(ctx, ruleID, userID)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden") // Check de custom error
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestRuleStore_DeleteRule(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		store, mockPool := setupRuleStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()

		// Verwacht een Exec call die 1 rij beïnvloedt
		mockPool.ExpectExec("^DELETE FROM automation_rules").
			WithArgs(ruleID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		// Act
		err := store.DeleteRule(ctx, ruleID)

		// Assert
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("Not Found", func(t *testing.T) {
		store, mockPool := setupRuleStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()

		// Verwacht een Exec call die 0 rijen beïnvloedt
		mockPool.ExpectExec("^DELETE FROM automation_rules").
			WithArgs(ruleID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		// Act
		err := store.DeleteRule(ctx, ruleID)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no rule found") // Check de custom error
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestRuleStore_UpdateRule(t *testing.T) {
	store, mockPool := setupRuleStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	ruleID := uuid.New()
	accountID := uuid.New()

	params := UpdateRuleParams{
		RuleID:            ruleID,
		Name:              "Updated Rule",
		TriggerConditions: json.RawMessage(`{"updated": true}`),
		ActionParams:      json.RawMessage(`{"action": "updated"}`),
	}

	// Mock de data die de DB teruggeeft na update
	rows := pgxmock.NewRows(ruleColumns).AddRow(
		ruleID, accountID, params.Name, true, // is_active blijft hetzelfde
		params.TriggerConditions, params.ActionParams, time.Now(), time.Now(),
	)

	mockPool.ExpectQuery("^UPDATE automation_rules").
		WithArgs(
			params.Name, params.TriggerConditions, params.ActionParams, params.RuleID,
		).
		WillReturnRows(rows)

	// Act
	rule, err := store.UpdateRule(ctx, params)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, ruleID, rule.ID)
	assert.Equal(t, "Updated Rule", rule.Name)
	assert.Equal(t, json.RawMessage(`{"updated": true}`), rule.TriggerConditions)
	assert.Equal(t, json.RawMessage(`{"action": "updated"}`), rule.ActionParams)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}
