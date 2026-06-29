package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zenflow/pkg/filter"
)

func TestBlockerMiddleware_EmptyHostFallback(t *testing.T) {
	domainBlocker := filter.NewBlocker([]string{"blocked.com", "malicious.org"})
	malwareBlocker := filter.NewBlocker([]string{"badware.net"})

	middleware := BlockerMiddleware(domainBlocker, malwareBlocker)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(dummyHandler)

	tests := []struct {
		name           string
		reqHost        string
		urlHost        string
		expectedStatus int
	}{
		{
			name:           "Allowed host, non-empty r.Host",
			reqHost:        "allowed.com",
			urlHost:        "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Blocked host in r.Host",
			reqHost:        "blocked.com",
			urlHost:        "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Blocked host in r.Host with port",
			reqHost:        "blocked.com:8080",
			urlHost:        "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Empty r.Host, allowed r.URL.Host",
			reqHost:        "",
			urlHost:        "allowed.com:8080",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Empty r.Host, blocked r.URL.Host in domain blocker",
			reqHost:        "",
			urlHost:        "blocked.com:8080",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Empty r.Host, blocked r.URL.Host in malware blocker",
			reqHost:        "",
			urlHost:        "badware.net:1234",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Empty r.Host, blocked r.URL.Host without port",
			reqHost:        "",
			urlHost:        "blocked.com",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://ignored", nil)
			req.Host = tt.reqHost
			if tt.urlHost != "" {
				req.URL.Host = tt.urlHost
			} else {
				req.URL.Host = tt.reqHost
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}
