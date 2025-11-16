package user

import (
	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/store"
	"net/http"

	"go.uber.org/zap" // <-- TOEGEVOEGD
)

// handleGetMe haalt de gegevens op van de ingelogde gebruiker.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleGetMe(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log) // <-- AANGEPAST
			return
		}

		user, err := store.GetUserByID(r.Context(), userID)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon gebruiker niet ophalen", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusOK, user, log) // <-- AANGEPAST
	}
}
