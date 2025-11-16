package database

import (
	"agenda-automator-api/db/migrations"
	"context"
	"errors"
	"testing"

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
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}
func (m *MockQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	panic("unimplemented")
}
func (m *MockQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	panic("unimplemented")
}

func TestRunMigrations_Success(t *testing.T) {
	// Arrange
	t.Setenv("RUN_MIGRATIONS", "true")
	ctx := context.Background()
	log := zap.NewNop()
	mockDB := new(MockQuerier)

	// Verwacht dat *elke* migratie wordt aangeroepen
	mockDB.On("Exec", ctx, migrations.InitialSchemaUp, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockDB.On("Exec", ctx, migrations.GmailSchemaUp, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockDB.On("Exec", ctx, migrations.OptimizationIndexesUp, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockDB.On("Exec", ctx, migrations.TableOptimizationsUp, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockDB.On("Exec", ctx, migrations.CalendarOptimizationsUp, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockDB.On(
		"Exec",
		ctx,
		migrations.ConnectedAccountsOptimizationUp,
		mock.Anything,
	).Return(pgconn.CommandTag{}, nil).Once()

	// Act
	err := RunMigrations(ctx, mockDB, log)

	// Assert
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestRunMigrations_Skip(t *testing.T) {
	// Arrange
	t.Setenv("RUN_MIGRATIONS", "false") // Env var staat uit
	ctx := context.Background()
	log := zap.NewNop()
	mockDB := new(MockQuerier)

	// We verwachten *geen* aanroepen naar Exec

	// Act
	err := RunMigrations(ctx, mockDB, log)

	// Assert
	assert.NoError(t, err)
	mockDB.AssertExpectations(t) // Verifieert dat Exec NIET is aangeroepen
}

func TestRunMigrations_Fail(t *testing.T) {
	// Arrange
	t.Setenv("RUN_MIGRATIONS", "true")
	ctx := context.Background()
	log := zap.NewNop()
	mockDB := new(MockQuerier)
	dbError := errors.New("DB migration failed")

	// Laat de *eerste* migratie falen
	mockDB.On("Exec", ctx, migrations.InitialSchemaUp, mock.Anything).Return(pgconn.CommandTag{}, dbError).Once()
	// We verwachten *niet* dat de andere migraties worden aangeroepen

	// Act
	err := RunMigrations(ctx, mockDB, log)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, dbError, err)
	mockDB.AssertExpectations(t)
}
