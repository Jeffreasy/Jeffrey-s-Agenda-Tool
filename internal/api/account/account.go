package account

import (
	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/store"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap" // <-- MOEST WORDEN TOEGEVOEGD
)

// HandleGetConnectedAccounts haalt alle gekoppelde accounts op voor de gebruiker.
// AANGEPAST: Accepteert nu *zap.Logger
func HandleGetConnectedAccounts(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			// AANGEPAST: log meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log)
			return
		}

		accounts, err := store.GetAccountsForUser(r.Context(), userID)
		if err != nil {
			// AANGEPAST: log meegegeven
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon accounts niet ophalen", log)
			return
		}

		// AANGEPAST: log meegegeven
		common.WriteJSON(w, http.StatusOK, accounts, log)
	}
}

// HandleDeleteConnectedAccount verwijdert een gekoppeld account.
// AANGEPAST: Accepteert nu *zap.Logger
func HandleDeleteConnectedAccount(store store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDStr := chi.URLParam(r, "accountId")
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			// AANGEPAST: log meegegeven
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig account ID", log)
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			// AANGEPAST: log meegegeven
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log)
			return
		}

		account, err := store.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			// AANGEPAST: log meegegeven
			common.WriteJSONError(w, http.StatusNotFound, "Account niet gevonden", log)
			return
		}

		err = store.DeleteConnectedAccount(r.Context(), accountID)
		if err != nil {
			// AANGEPAST: log meegegeven
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon account niet verwijderen", log)
			return
		}

		// AANGEPAST: log meegegeven
		common.WriteJSON(w, http.StatusNoContent, nil, log)
	}
}
