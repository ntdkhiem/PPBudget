package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/middleware"

	"github.com/go-chi/chi/v5"
)

const (
	testAccountID  = "7d0f0f6e-2b1c-4d7a-9c2e-3f5b8a1d4e60"
	testSnapshotID = "0a9b8c7d-6e5f-4a3b-8c2d-1e0f9a8b7c6d"
)

// newBalanceRequest builds a request carrying an authenticated user id and chi
// route params, matching what the router provides in production.
func newBalanceRequest(method, target string, body io.Reader, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, body)

	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.UserIDKey, "test-user")
	return req.WithContext(ctx)
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON error: %q", rec.Body.String())
	}
	return body.Error
}

// These paths must fail validation before touching the (nil) service.
func TestBalanceHandlers_Validation(t *testing.T) {
	h := New(nil, slog.New(slog.NewJSONHandler(io.Discard, nil)), &config.Config{})

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
		{"post invalid JSON", h.RecordManualBalance, http.MethodPost, "not json", map[string]string{"id": testAccountID}, true, 400, "invalid payload"},
		{"post missing balance", h.RecordManualBalance, http.MethodPost, `{"as_of_date":"2026-01-01"}`, map[string]string{"id": testAccountID}, true, 400, "balance is required"},
		{"post bad date format", h.RecordManualBalance, http.MethodPost, `{"balance":1000,"as_of_date":"01/01/2026"}`, map[string]string{"id": testAccountID}, true, 400, "invalid as_of_date format, expected YYYY-MM-DD"},
		{"post missing account id", h.RecordManualBalance, http.MethodPost, `{"balance":1000}`, map[string]string{"id": ""}, true, 400, "account id is required"},
		{"post non-uuid account id", h.RecordManualBalance, http.MethodPost, `{"balance":1000}`, map[string]string{"id": "acc1"}, true, 400, "invalid account id"},
		{"get non-uuid account id", h.ListBalanceSnapshots, http.MethodGet, "", map[string]string{"id": "abc"}, true, 400, "invalid account id"},
		{"delete non-uuid account id", h.DeleteBalanceSnapshot, http.MethodDelete, "", map[string]string{"id": "abc", "snapshotId": testSnapshotID}, true, 400, "invalid account id"},
		{"delete non-uuid snapshot id", h.DeleteBalanceSnapshot, http.MethodDelete, "", map[string]string{"id": testAccountID, "snapshotId": "1; DROP TABLE x"}, true, 400, "invalid snapshot id"},
		{"delete missing snapshot id", h.DeleteBalanceSnapshot, http.MethodDelete, "", map[string]string{"id": testAccountID, "snapshotId": ""}, true, 400, "snapshot id is required"},
		{"post unauthenticated", h.RecordManualBalance, http.MethodPost, `{"balance":1000}`, map[string]string{"id": testAccountID}, false, 401, "unauthorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.authed {
				req = newBalanceRequest(tt.method, "/api/v1/accounts/x/balances", bytes.NewBufferString(tt.body), tt.params)
			} else {
				req = httptest.NewRequest(tt.method, "/api/v1/accounts/x/balances", bytes.NewBufferString(tt.body))
			}
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := errorMessage(t, rec); got != tt.wantError {
				t.Errorf("error = %q, want %q", got, tt.wantError)
			}
		})
	}
}
