package gmail

import (
	"context"
	"errors"
	"testing"
	"unsafe"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	str := args.Get(0).(string)
	return *(*pgconn.CommandTag)(unsafe.Pointer(&str)), args.Error(1)
}
func (m *MockQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	// Implementeer dit als je GetGmailRulesForAccount wilt testen
	panic("unimplemented")
}

func (m *MockQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	// Implementeer dit als je CreateGmailAutomationRule wilt testen
	panic("unimplemented")
}

// --- De Test ---

func TestGmailStore_DeleteGmailRule(t *testing.T) {
	ctx := context.Background()
	testRuleID := uuid.New()
	testLogger := zap.NewNop()

	t.Run("Success", func(t *testing.T) {
		// Arrange
		mockDB := new(MockQuerier)
		store := NewGmailStore(mockDB, testLogger)

		// We verwachten dat "Exec" wordt aangeroepen met de juiste query en ID.
		// We mocken de return value: een CommandTag die zegt "1 row affected" en geen error.
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), []interface{}{testRuleID}).
			Return("DELETE 1", nil).
			Once() // Verwacht dat het n keer wordt aangeroepen

		// Act
		err := store.DeleteGmailRule(ctx, testRuleID)

		// Assert
		assert.NoError(t, err)
		mockDB.AssertExpectations(t) // Controleer of aan alle "On" verwachtingen is voldaan
	})

	t.Run("Error from DB", func(t *testing.T) {
		// Arrange
		mockDB := new(MockQuerier)
		store := NewGmailStore(mockDB, testLogger)
		dbError := errors.New("database connection lost")

		// We simuleren een database error
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), []interface{}{testRuleID}).
			Return("", dbError).
			Once()

		// Act
		err := store.DeleteGmailRule(ctx, testRuleID)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, dbError, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("Not Found (0 rows affected)", func(t *testing.T) {
		// Arrange
		mockDB := new(MockQuerier)
		store := NewGmailStore(mockDB, testLogger)

		// We simuleren een succesvolle query, maar die 0 rijen heeft verwijderd
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), []interface{}{testRuleID}).
			Return("DELETE 0", nil).
			Once()

		// Act
		err := store.DeleteGmailRule(ctx, testRuleID)

		// Assert
		assert.Error(t, err) // Je code retourneert een error in dit geval
		assert.Contains(t, err.Error(), "no Gmail rule found")
		mockDB.AssertExpectations(t)
	})
}
