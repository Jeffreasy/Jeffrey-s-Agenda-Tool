package rule

import (
	"agenda-automator-api/internal/api/common"
	"agenda-automator-api/internal/domain"
	"agenda-automator-api/internal/store"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap" // <-- TOEGEVOEGD
)

// handleCreateRule creert een nieuwe automation rule.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleCreateRule(storer store.Storer, log *zap.Logger) http.HandlerFunc {
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

		account, err := storer.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusNotFound, "Account niet gevonden", log) // <-- AANGEPAST
			return
		}

		var req domain.AutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", log) // <-- AANGEPAST
			return
		}

		params := store.CreateAutomationRuleParams{
			ConnectedAccountID: accountID,
			Name:               req.Name,
			TriggerConditions:  req.TriggerConditions,
			ActionParams:       req.ActionParams,
		}

		rule, err := storer.CreateAutomationRule(r.Context(), params)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet creren", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusCreated, rule, log) // <-- AANGEPAST
	}
}

// handleGetRules haalt alle rules op voor een account.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleGetRules(storer store.Storer, log *zap.Logger) http.HandlerFunc {
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

		account, err := storer.GetConnectedAccountByID(r.Context(), accountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusNotFound, "Account niet gevonden", log) // <-- AANGEPAST
			return
		}

		rules, err := storer.GetRulesForAccount(r.Context(), accountID)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon rules niet ophalen", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusOK, rules, log) // <-- AANGEPAST
	}
}

// handleUpdateRule update een bestaande rule.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleUpdateRule(storer store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID", log) // <-- AANGEPAST
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log) // <-- AANGEPAST
			return
		}

		rule, err := storer.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			common.WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden", log) // <-- AANGEPAST
			return
		}

		account, err := storer.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule", log) // <-- AANGEPAST
			return
		}

		var req domain.AutomationRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldige request body", log) // <-- AANGEPAST
			return
		}

		params := store.UpdateRuleParams{
			RuleID:            ruleID,
			Name:              req.Name,
			TriggerConditions: req.TriggerConditions,
			ActionParams:      req.ActionParams,
		}

		updatedRule, err := storer.UpdateRule(r.Context(), params)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet updaten", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusOK, updatedRule, log) // <-- AANGEPAST
	}
}

// handleDeleteRule verwijdert een rule.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleDeleteRule(storer store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID", log) // <-- AANGEPAST
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log) // <-- AANGEPAST
			return
		}

		rule, err := storer.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			common.WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden", log) // <-- AANGEPAST
			return
		}

		account, err := storer.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule", log) // <-- AANGEPAST
			return
		}

		err = storer.DeleteRule(r.Context(), ruleID)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon rule niet verwijderen", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusNoContent, nil, log) // <-- AANGEPAST
	}
}

// handleToggleRule togglet de active status van een rule.
// AANGEPAST: Accepteert nu log *zap.Logger
func HandleToggleRule(storer store.Storer, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleIDStr := chi.URLParam(r, "ruleId")
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			common.WriteJSONError(w, http.StatusBadRequest, "Ongeldig rule ID", log) // <-- AANGEPAST
			return
		}

		userID, err := common.GetUserIDFromContext(r.Context())
		if err != nil {
			common.WriteJSONError(w, http.StatusUnauthorized, err.Error(), log) // <-- AANGEPAST
			return
		}

		rule, err := storer.GetRuleByID(r.Context(), ruleID)
		if err != nil {
			common.WriteJSONError(w, http.StatusNotFound, "Rule niet gevonden", log) // <-- AANGEPAST
			return
		}

		account, err := storer.GetConnectedAccountByID(r.Context(), rule.ConnectedAccountID)
		if err != nil || account.UserID != userID {
			common.WriteJSONError(w, http.StatusForbidden, "Geen toegang tot deze rule", log) // <-- AANGEPAST
			return
		}

		updatedRule, err := storer.ToggleRuleStatus(r.Context(), ruleID)
		if err != nil {
			common.WriteJSONError(w, http.StatusInternalServerError, "Kon rule status niet togglen", log) // <-- AANGEPAST
			return
		}

		common.WriteJSON(w, http.StatusOK, updatedRule, log) // <-- AANGEPAST
	}
}
