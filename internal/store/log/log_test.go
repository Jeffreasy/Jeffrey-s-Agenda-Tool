package log

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"agenda-automator-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLogStore is een helper die een LogStore en een mock pool aanmaakt.
func setupLogStore(t *testing.T) (LogStorer, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)

	// De NewLogStore accepteert al een interface, perfect!
	store := NewLogStore(mockPool)
	return store, mockPool
}

func TestLogStore_CreateAutomationLog(t *testing.T) {
	store, mockPool := setupLogStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	ruleID := uuid.New()
	params := CreateLogParams{
		ConnectedAccountID: uuid.New(),
		RuleID:             &ruleID,
		Status:             domain.LogSuccess,
		TriggerDetails:     json.RawMessage(`{}`),
		ActionDetails:      json.RawMessage(`{}`),
		ErrorMessage:       "",
	}

	// Dit is een simpele INSERT zonder RETURNING, dus we verwachten Exec
	mockPool.ExpectExec("INSERT INTO automation_logs").
		WithArgs(
			params.ConnectedAccountID,
			params.RuleID,
			params.Status,
			params.TriggerDetails,
			params.ActionDetails,
			params.ErrorMessage,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Act
	err := store.CreateAutomationLog(ctx, params)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestLogStore_CreateAutomationLog_Error(t *testing.T) {
	store, mockPool := setupLogStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	ruleID := uuid.New()
	params := CreateLogParams{
		ConnectedAccountID: uuid.New(),
		RuleID:             &ruleID,
		Status:             domain.LogSuccess,
		TriggerDetails:     json.RawMessage(`{}`),
		ActionDetails:      json.RawMessage(`{}`),
		ErrorMessage:       "",
	}

	// Simuleer een database error
	dbError := errors.New("connection failed")
	mockPool.ExpectExec("INSERT INTO automation_logs").
		WithArgs(
			params.ConnectedAccountID,
			params.RuleID,
			params.Status,
			params.TriggerDetails,
			params.ActionDetails,
			params.ErrorMessage,
		).
		WillReturnError(dbError)

	// Act
	err := store.CreateAutomationLog(ctx, params)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, dbError, err)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestLogStore_HasLogForTrigger(t *testing.T) {
	t.Run("Log exists", func(t *testing.T) {
		store, mockPool := setupLogStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()
		eventID := "google-event-id"

		// Mock de data die de DB teruggeeft (1 rij met de waarde '1')
		rows := pgxmock.NewRows([]string{"1"}).AddRow(1)

		mockPool.ExpectQuery("SELECT 1").
			WithArgs(ruleID, eventID).
			WillReturnRows(rows)

		// Act
		exists, err := store.HasLogForTrigger(ctx, ruleID, eventID)

		// Assert
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("Log does not exist", func(t *testing.T) {
		store, mockPool := setupLogStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()
		eventID := "google-event-id"

		// Simuleer een "no rows" error
		mockPool.ExpectQuery("SELECT 1").
			WithArgs(ruleID, eventID).
			WillReturnError(pgx.ErrNoRows)

		// Act
		exists, err := store.HasLogForTrigger(ctx, ruleID, eventID)

		// Assert
		assert.NoError(t, err)  // De functie zelf hoort geen error te geven
		assert.False(t, exists) // Het moet false teruggeven
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("Database error", func(t *testing.T) {
		store, mockPool := setupLogStore(t)
		defer mockPool.Close()

		ctx := context.Background()
		ruleID := uuid.New()
		eventID := "google-event-id"
		dbError := errors.New("connection failed")

		// Simuleer een willekeurige database error
		mockPool.ExpectQuery("SELECT 1").
			WithArgs(ruleID, eventID).
			WillReturnError(dbError)

		// Act
		exists, err := store.HasLogForTrigger(ctx, ruleID, eventID)

		// Assert
		assert.Error(t, err) // Nu verwachten we wel een error
		assert.Equal(t, dbError, err)
		assert.False(t, exists)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestLogStore_GetLogsForAccount(t *testing.T) {
	store, mockPool := setupLogStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	accountID := uuid.New()
	ruleID := uuid.New()
	limit := 10

	// Definieer de kolommen
	logColumns := []string{
		"id", "connected_account_id", "rule_id", "timestamp", "status",
		"trigger_details", "action_details", "error_message",
	}

	// Maak een mock rij
	rows := pgxmock.NewRows(logColumns).AddRow(
		int64(1), accountID, &ruleID, time.Now(), domain.LogSuccess,
		json.RawMessage(`{}`), json.RawMessage(`{}`), "",
	)

	mockPool.ExpectQuery("SELECT id, connected_account_id").
		WithArgs(accountID, limit).
		WillReturnRows(rows)

	// Act
	logs, err := store.GetLogsForAccount(ctx, accountID, limit)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, int64(1), logs[0].ID)
	assert.Equal(t, domain.LogSuccess, logs[0].Status)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestLogStore_GetLogsForAccount_Error(t *testing.T) {
	store, mockPool := setupLogStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	accountID := uuid.New()
	limit := 10

	// Simuleer een database error
	dbError := errors.New("query failed")
	mockPool.ExpectQuery("SELECT id, connected_account_id").
		WithArgs(accountID, limit).
		WillReturnError(dbError)

	// Act
	logs, err := store.GetLogsForAccount(ctx, accountID, limit)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, dbError, err)
	assert.Nil(t, logs)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestLogStore_GetLogsForAccount_ScanError(t *testing.T) {
	store, mockPool := setupLogStore(t)
	defer mockPool.Close()

	ctx := context.Background()
	accountID := uuid.New()
	limit := 10

	// Definieer de kolommen
	logColumns := []string{
		"id", "connected_account_id", "rule_id", "timestamp", "status",
		"trigger_details", "action_details", "error_message",
	}

	// Maak een mock rij met een probleem dat een scan error zou veroorzaken
	rows := pgxmock.NewRows(logColumns).AddRow("invalid", uuid.New(), nil, time.Now(), domain.LogSuccess, json.RawMessage(`{}`), json.RawMessage(`{}`), "")

	mockPool.ExpectQuery("SELECT id, connected_account_id").
		WithArgs(accountID, limit).
		WillReturnRows(rows)

	// Act
	logs, err := store.GetLogsForAccount(ctx, accountID, limit)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, logs)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}
