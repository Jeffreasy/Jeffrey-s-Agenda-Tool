package log

import (
	"net/http"

	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap" // <-- TOEGEVOEGD
)

// HandleGetAutomationLogs haalt logs op voor een account.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleGetAutomationLogs(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log) // <-- AANGEPAST
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log) // <-- AANGEPAST
			return
		}

		account, err := store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusNotFound, "Account niet gevonden", log) // <-- AANGEPAST
			return
		}

		limit := 50 // Default limit
		logs, err := store.GetLogsForAccount(r.Context(), accountID, limit)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon logs niet ophalen", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusOK, logs, log) // <-- AANGEPAST
	}
}
