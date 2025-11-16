package worker

import (
	"testing"

	"agenda-automator-api/internal/store"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap" // <-- TOEGEVOEGD
)

func TestNewWorker(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a mock store
	mockStore := &store.MockStore{}

	// Test constructor
	// AANGEPAST: testLogger meegegeven
	worker, err := NewWorker(mockStore, testLogger)
	assert.NoError(t, err)
	assert.NotNil(t, worker)
	assert.Equal(t, mockStore, worker.store)
	assert.Equal(t, testLogger, worker.logger) // <-- AANGEPAST
	assert.NotNil(t, worker.calendarProcessor)
	assert.NotNil(t, worker.gmailProcessor)
}

func TestNewWorker_NilStore(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Test constructor with nil store
	// AANGEPAST: testLogger meegegeven
	worker, err := NewWorker(nil, testLogger)
	assert.NoError(t, err)
	assert.NotNil(t, worker)
	assert.Nil(t, worker.store)
	assert.Equal(t, testLogger, worker.logger) // <-- AANGEPAST
	assert.NotNil(t, worker.calendarProcessor)
	assert.NotNil(t, worker.gmailProcessor)
}

// Note: Testing the Start() method and background processing is complex
// because it involves goroutines and timing. In a real scenario, you would
// use integration tests or more sophisticated testing approaches to verify
// the worker's behavior over time.
