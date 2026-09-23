package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/internal/middleware"
)

// Wealth Strategy endpoints.
//
// These replace GET|PUT /settings/values/{key} as the way planning data is
// stored. That route accepts any key and any string with no validation, which
// was tolerable while two read-only pages rendered the result and is not once a
// deterministic engine reads it to decide what to tell someone to do with their
// money.

// GetWealthProfile returns the profile with per-field provenance.
func (h *Handler) GetWealthProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	profile, err := h.svc.GetWealthProfile(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get wealth profile", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get wealth profile")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// updateWealthProfileRequest carries a partial update plus the provenance of
// the values in it.
type updateWealthProfileRequest struct {
	// Source says how these values arrived: 'entered' when typed, 'confirmed'
	// when a derived figure was shown and accepted, 'derived' when computed
	// without being seen. Defaults to 'entered'.
	//
	// This is not bookkeeping. The generator refuses to run high-stakes
	// branches -- the backdoor Roth decision above all -- off a value the user
	// never actually looked at, so claiming the wrong source here is a
	// correctness bug rather than a cosmetic one.
	Source string `json:"source"`
	// Fields is the partial profile. Only keys physically present in the JSON
	// are written.
	Fields json.RawMessage `json:"fields"`
}

// UpdateWealthProfile applies a partial update.
//
// The body is decoded TWICE, deliberately: once into a map to learn which keys
// the client actually sent, and once into the typed profile for their values.
// Go cannot otherwise distinguish "field omitted" from "field explicitly set to
// null", and those mean opposite things here -- the first must leave the stored
// answer alone, the second must clear it. Inferring intent from nil-ness would
// make an answer impossible to erase and would silently rewrite every field on
// every partial save.
func (h *Handler) UpdateWealthProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req updateWealthProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if len(req.Fields) == 0 {
		writeError(w, http.StatusBadRequest, "no fields supplied")
		return
	}

	var present map[string]json.RawMessage
	if err := json.Unmarshal(req.Fields, &present); err != nil {
		writeError(w, http.StatusBadRequest, "fields must be an object")
		return
	}

	var profile domain.WealthProfile
	if err := json.Unmarshal(req.Fields, &profile); err != nil {
		writeError(w, http.StatusBadRequest, "one or more field values have the wrong type")
		return
	}

	// Reject unknown keys rather than dropping them. A typo that silently
	// succeeds leaves the user believing they answered something they did not,
	// and the action it was meant to unlock stays mysteriously blocked.
	setKeys := make([]string, 0, len(present))
	for key := range present {
		if !h.svc.IsProfileField(key) {
			writeError(w, http.StatusBadRequest, "unknown profile field: "+key)
			return
		}
		setKeys = append(setKeys, key)
	}
	sort.Strings(setKeys) // deterministic write order, so tests and logs are stable

	source := req.Source
	if source == "" {
		source = domain.FieldSourceEntered
	}

	updated, err := h.svc.SaveWealthProfile(r.Context(), userID, &profile, setKeys, source)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("failed to save wealth profile", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save wealth profile")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// GetDerivedProfile returns the values the ledger can answer on the user's
// behalf, for the confirmation screen.
//
// Nothing is written here. Accepting a value is a separate PUT with source
// 'confirmed', which is what distinguishes a figure the user actually saw from
// one the app inferred quietly.
func (h *Handler) GetDerivedProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	derived, err := h.svc.DeriveProfileValues(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to derive profile values", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to derive profile values")
		return
	}
	writeJSON(w, http.StatusOK, derived)
}

// GetWealthFields returns every recognised profile field key.
//
// The intake UI uses this to build its question list from the same registry the
// engine validates against, so the two cannot drift.
func (h *Handler) GetWealthFields(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fields": h.svc.ProfileFieldKeys(),
		// Labels cover the pseudo-keys too (transaction_history, paystub_ytd),
		// which name data the app lacks rather than questions for the user --
		// the client tells them apart by checking membership in "fields".
		"labels": h.svc.FieldLabels(),
	})
}

// ------------------------------------------------------------------ quests

// ListQuests returns the user's generated action list.
//
// Regenerates on read. The interpolated figures -- "clear the $3,824 balance" --
// go stale as balances move, and an action list quoting last week's numbers is
// worse than one that recomputes, because the user has no way to tell which.
// Completion survives regeneration; see ReplaceQuests.
func (h *Handler) ListQuests(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	quests, summary, err := h.svc.ListQuests(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list quests", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to build your action list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"quests":  quests,
		"phases":  h.svc.Phases(),
		"summary": summary,
	})
}

// UpdateQuestStatus records a completion, a skip, or a reversal.
//
// Until auto-verification lands, every completion arrives here as a manual
// claim. The source is recorded rather than assumed so that when the evaluator
// does start closing actions on its own, the two remain distinguishable in the
// history -- an auto-completion that later proves wrong needs to be traceable.
func (h *Handler) UpdateQuestStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	questID := chi.URLParam(r, "id")

	var body struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	quest, err := h.svc.SetQuestStatus(r.Context(), userID, questID,
		body.Status, domain.QuestSourceManual, body.Note)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound):
			writeError(w, http.StatusNotFound, "action not found")
		case errors.Is(err, apperrors.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("failed to set quest status", "user_id", userID, "quest_id", questID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update the action")
		}
		return
	}
	writeJSON(w, http.StatusOK, quest)
}

// ListQuestEvents returns one action's history.
func (h *Handler) ListQuestEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	events, err := h.svc.ListQuestEvents(r.Context(), userID, chi.URLParam(r, "id"))
	if err != nil {
		h.logger.Error("failed to list quest events", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load the action history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// ------------------------------------------------------- paystub and terms

// GetPaystub returns the most recent year-to-date payroll figures, or null.
//
// Null is a normal state, not an error: it means the user has not entered a
// stub yet, and several actions deliberately decline to state a figure until
// they have one rather than treating absent as zero.
func (h *Handler) GetPaystub(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	paystub, err := h.svc.GetLatestPaystub(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get paystub", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get paystub")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paystub": paystub})
}

// UpsertPaystub records one dated year-to-date reading.
//
// Dated rather than overwritten: a user who changes their deferral rate
// mid-year produces two rows whose difference reveals the change, where a
// single mutable row would hide it and mis-pace the rest of the year.
func (h *Handler) UpsertPaystub(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var p domain.PaystubYTD
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if p.AsOfDate.IsZero() {
		writeError(w, http.StatusBadRequest, "as_of_date is required")
		return
	}

	if err := h.svc.UpsertPaystub(r.Context(), userID, p); err != nil {
		if errors.Is(err, apperrors.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("failed to save paystub", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save paystub")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListAccountTerms returns the per-account facts an aggregator does not supply:
// interest rates above all, which is what lets balances be ordered against each
// other and against investing.
func (h *Handler) ListAccountTerms(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	terms, err := h.svc.ListAccountTerms(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list account terms", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list account terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"terms": terms})
}

// UpsertAccountTerms records the terms for one account.
func (h *Handler) UpsertAccountTerms(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var terms domain.AccountTerms
	if err := json.NewDecoder(r.Body).Decode(&terms); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	terms.AccountID = chi.URLParam(r, "id")

	if err := h.svc.UpsertAccountTerms(r.Context(), userID, terms); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrForbidden), errors.Is(err, apperrors.ErrNotFound):
			writeError(w, http.StatusNotFound, "account not found")
		case errors.Is(err, apperrors.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("failed to save account terms", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save account terms")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ------------------------------------------------------------------- goals

// ListGoals returns the user's savings goals.
func (h *Handler) ListGoals(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	goals, err := h.svc.ListGoals(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list goals", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list goals")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"goals": goals})
}

// UpsertGoal creates or updates one goal.
//
// Goals are the only user-authored content in this feature -- a name, an amount
// and a date that came from the person rather than from their transactions --
// so this is the one write here that destroys something irreplaceable if it
// goes wrong.
func (h *Handler) UpsertGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var g domain.Goal
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if id := chi.URLParam(r, "id"); id != "" {
		g.ID = id
	}
	if g.Name == "" {
		writeError(w, http.StatusBadRequest, "a goal needs a name")
		return
	}
	if g.TargetAmount <= 0 {
		writeError(w, http.StatusBadRequest, "a goal needs a target above zero")
		return
	}

	id, err := h.svc.UpsertGoal(r.Context(), userID, g)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrNotFound), errors.Is(err, apperrors.ErrForbidden):
			writeError(w, http.StatusNotFound, "goal not found")
		case errors.Is(err, apperrors.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("failed to save goal", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save goal")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// DeleteGoal removes one goal.
func (h *Handler) DeleteGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.DeleteGoal(r.Context(), userID, chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "goal not found")
			return
		}
		h.logger.Error("failed to delete goal", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete goal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListRetirementAccountTerms maps the user's accounts to tax treatments.
func (h *Handler) ListRetirementAccountTerms(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	terms, err := h.svc.ListRetirementAccountTerms(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list retirement account terms", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list retirement accounts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"terms": terms})
}

// UpsertRetirementAccountTerms records an account's tax treatment and
// contribution, replacing the per-account entries the retirement_plan blob held.
func (h *Handler) UpsertRetirementAccountTerms(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var terms domain.RetirementAccountTerms
	if err := json.NewDecoder(r.Body).Decode(&terms); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	terms.AccountID = chi.URLParam(r, "id")

	if err := h.svc.UpsertRetirementAccountTerms(r.Context(), userID, terms); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrForbidden), errors.Is(err, apperrors.ErrNotFound):
			writeError(w, http.StatusNotFound, "account not found")
		case errors.Is(err, apperrors.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("failed to save retirement account terms", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save retirement account")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteRetirementAccountTerms unmaps an account from its tax treatment.
func (h *Handler) DeleteRetirementAccountTerms(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.DeleteRetirementAccountTerms(r.Context(), userID, chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		h.logger.Error("failed to delete retirement account terms", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove retirement account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
