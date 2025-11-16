package log

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

func TestNewLogStore(t *testing.T) {
	// Test constructor with nil pool
	store := NewLogStore(nil)
	assert.NotNil(t, store)
	assert.Nil(t, store.pool)

	// Test constructor with a pool
	pool := &pgxpool.Pool{}
	store = NewLogStore(pool)
	assert.NotNil(t, store)
	assert.Equal(t, pool, store.pool)
}

