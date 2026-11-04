package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRejectUntrustedOrigins(t *testing.T) {
	handler := RejectUntrustedOrigins(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{name: "native request", wantStatus: http.StatusNoContent},
		{name: "wails origin", origin: "http://wails.localhost", wantStatus: http.StatusNoContent},
		{name: "development origin", origin: "http://localhost:34115", wantStatus: http.StatusNoContent},
		{name: "untrusted origin", origin: "https://example.com", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
