package handler

import (
	"crypto/subtle"
	"net/http"
)

// DeadlineRemindersCronTrigger runs the deadline reminder job on demand, for
// the scheduled workflow that stands in for the in-process cron on hosts that
// sleep. Protected by X-API-Key, like the SimpleFIN trigger. Running it twice
// in a day sends nothing new.
func (h *Handler) DeadlineRemindersCronTrigger(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" || subtle.ConstantTimeCompare([]byte(apiKey), []byte(h.cfg.IngestAPIKey)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	sent, err := h.svc.SendDeadlineReminders(r.Context())
	if err != nil {
		// Some users' reminders may still have gone out; the rest are retried
		// on the next run, since only sent ones are recorded.
		h.logger.Error("cron triggered deadline reminders failed", "error", err, "sent", sent)
		writeError(w, http.StatusInternalServerError, "deadline reminders failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "sent": sent})
}
