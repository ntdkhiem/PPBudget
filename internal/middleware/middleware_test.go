package middleware

import (
	
	
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestRequireJWT(t *testing.T) {
	secret := "test-secret-key-12345"

	// Helper to generate token
	generateToken := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(secret))
		return tokenString
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserID(r.Context())
		if !ok || userID == "" {
			http.Error(w, "user_id not in context", http.StatusInternalServerError)
			return
		}
		rawUserID := r.Context().Value("user_id")
		if rawUserID != userID {
			http.Error(w, "raw user_id mismatch", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok:" + userID))
	})

	mw := RequireJWT(secret)(testHandler)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid bearer token format",
			authHeader:     "Bearer invalid.token.value",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "token missing user_id claim",
			authHeader: "Bearer " + generateToken(jwt.MapClaims{
				"role": "admin",
				"exp":  time.Now().Add(time.Hour).Unix(),
			}),
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "valid token with user_id",
			authHeader: "Bearer " + generateToken(jwt.MapClaims{
				"user_id": "usr-123e4567-e89b-12d3-a456-426614174000",
				"exp":     time.Now().Add(time.Hour).Unix(),
			}),
			expectedStatus: http.StatusOK,
			expectedBody:   "ok:usr-123e4567-e89b-12d3-a456-426614174000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			mw.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d, body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			if tt.expectedBody != "" && rec.Body.String() != tt.expectedBody {
				t.Fatalf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}
