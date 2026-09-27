package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ntdkhiem/ppbudget-go/internal/config"
)

// These paths must fail validation before touching the (nil) service.
func TestPlannedExpenseHandlers_Validation(t *testing.T) {
	h := New(nil, slog.New(slog.NewJSONHandler(io.Discard, nil)), &config.Config{})
	const date = `"target_date":"2027-03-15T00:00:00Z"`
	long := strings.Repeat("x", 121)

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		method     string
		body       string
		params     map[string]string
		authed     bool
		wantStatus int
		wantError  string
	}{
		{"create invalid JSON", h.UpsertPlannedExpense, http.MethodPost, "not json", nil, true, 400, "invalid payload"},
		{"create with a bare date", h.UpsertPlannedExpense, http.MethodPost, `{"label":"Trip","amount":100,"target_date":"2027-03-15"}`, nil, true, 400, "invalid payload"},
		{"create without a label", h.UpsertPlannedExpense, http.MethodPost, `{"label":"  ","amount":100,` + date + `}`, nil, true, 400, "a planned expense needs a label"},
		{"create with a long label", h.UpsertPlannedExpense, http.MethodPost, `{"label":"` + long + `","amount":100,` + date + `}`, nil, true, 400, "keep the label under 120 characters"},
		{"create with a zero amount", h.UpsertPlannedExpense, http.MethodPost, `{"label":"Trip","amount":0,` + date + `}`, nil, true, 400, "a planned expense needs an amount above zero"},
		{"create with a negative amount", h.UpsertPlannedExpense, http.MethodPost, `{"label":"Trip","amount":-5,` + date + `}`, nil, true, 400, "a planned expense needs an amount above zero"},
		{"create without a date", h.UpsertPlannedExpense, http.MethodPost, `{"label":"Trip","amount":100}`, nil, true, 400, "a planned expense needs a date"},
		{"update non-uuid id", h.UpsertPlannedExpense, http.MethodPut, `{"label":"Trip","amount":100,` + date + `}`, map[string]string{"id": "abc"}, true, 400, "invalid planned expense id"},
		{"delete non-uuid id", h.DeletePlannedExpense, http.MethodDelete, "", map[string]string{"id": "1; DROP TABLE x"}, true, 400, "invalid planned expense id"},
		{"delete missing id", h.DeletePlannedExpense, http.MethodDelete, "", map[string]string{"id": ""}, true, 400, "planned expense id is required"},
		{"create unauthenticated", h.UpsertPlannedExpense, http.MethodPost, `{"label":"Trip"}`, nil, false, 401, "unauthorized"},
		{"list unauthenticated", h.ListPlannedExpenses, http.MethodGet, "", nil, false, 401, "unauthorized"},
		{"delete unauthenticated", h.DeletePlannedExpense, http.MethodDelete, "", map[string]string{"id": testGrantID}, false, 401, "unauthorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.authed {
				req = newBalanceRequest(tt.method, "/", strings.NewReader(tt.body), tt.params)
			} else {
				req = httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			}
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := errorMessage(t, rec); got != tt.wantError {
				t.Errorf("error: got %q want %q", got, tt.wantError)
			}
		})
	}
}
