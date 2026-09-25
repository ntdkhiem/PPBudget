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

const testGrantID = "5c1d2e3f-4a5b-4c6d-8e7f-9a0b1c2d3e4f"

// These paths must fail validation before touching the (nil) service.
func TestEquityGrantHandlers_Validation(t *testing.T) {
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
		{"create invalid JSON", h.UpsertEquityGrant, http.MethodPost, "not json", nil, true, 400, "invalid payload"},
		{"create without a kind", h.UpsertEquityGrant, http.MethodPost, `{"label":"Grant"}`, nil, true, 400, "kind must be one of rsu, espp, iso or nso"},
		{"create with an unknown kind", h.UpsertEquityGrant, http.MethodPost, `{"kind":"warrant"}`, nil, true, 400, "kind must be one of rsu, espp, iso or nso"},
		{"update non-uuid id", h.UpsertEquityGrant, http.MethodPut, `{"kind":"rsu"}`, map[string]string{"id": "abc"}, true, 400, "invalid grant id"},
		{"delete non-uuid id", h.DeleteEquityGrant, http.MethodDelete, "", map[string]string{"id": "1; DROP TABLE x"}, true, 400, "invalid grant id"},
		{"delete missing id", h.DeleteEquityGrant, http.MethodDelete, "", map[string]string{"id": ""}, true, 400, "grant id is required"},
		{"create unauthenticated", h.UpsertEquityGrant, http.MethodPost, `{"kind":"rsu"}`, nil, false, 401, "unauthorized"},
		{"list unauthenticated", h.ListEquityGrants, http.MethodGet, "", nil, false, 401, "unauthorized"},
		{"delete unauthenticated", h.DeleteEquityGrant, http.MethodDelete, "", map[string]string{"id": testGrantID}, false, 401, "unauthorized"},
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
