package user

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

func TestNewUserStore(t *testing.T) {
	// Test constructor with nil pool
	store := NewUserStore(nil)
	assert.NotNil(t, store)
	assert.Nil(t, store.pool)

	// Test constructor with a pool
	pool := &pgxpool.Pool{}
	store = NewUserStore(pool)
	assert.NotNil(t, store)
	assert.Equal(t, pool, store.pool)
}

// Note: Testing methods with nil pool would require database mocking,
// which is complex for pgx. In a real scenario, you'd use integration tests
// with a test database or more sophisticated mocking libraries.

// Note: Full integration tests with a real database would be more comprehensive,
// but these tests verify the basic structure and nil-safety of the UserStore.
