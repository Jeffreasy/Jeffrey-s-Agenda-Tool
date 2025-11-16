package calendar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCalendarProcessor(t *testing.T) {
	// Test constructor with nil store
	processor := NewCalendarProcessor(nil)
	assert.NotNil(t, processor)
	assert.Nil(t, processor.store)

	// Test constructor with a mock store would require implementing the interface
	// For now, just test nil case
}

