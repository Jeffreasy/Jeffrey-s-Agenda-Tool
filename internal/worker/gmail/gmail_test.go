package gmail

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGmailProcessor(t *testing.T) {
	// Test constructor with nil store
	processor := NewGmailProcessor(nil)
	assert.NotNil(t, processor)
	assert.Nil(t, processor.store)

	// Test constructor with a mock store would require implementing the interface
	// For now, just test nil case
}

