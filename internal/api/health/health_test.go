package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap" // <-- TOEGEVOEGD
)

func TestHandleHealth(t *testing.T) {
	// AANGEPAST: Maak een test-logger
	testLogger := zap.NewNop()

	// Create a new HTTP request
	req, err := http.NewRequest("GET", "/api/v1/health", nil)
	assert.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Call the handler
	// AANGEPAST: testLogger meegegeven
	handler := HandleHealth(testLogger)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])

	// Check the content type
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}
