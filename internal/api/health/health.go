package health

import (
	"agenda-automator-api/internal/api/common"
	"net/http"

	"go.uber.org/zap" // <-- TOEGEVOEGD
)

// HandleHealth checks if the API server is running and healthy.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleHealth(log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// AANGEPAST: log meegegeven
		common.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"}, log)
	}
}
