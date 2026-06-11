package netutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractRawIP_SpoofingPrevention(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		expectedResult string
	}{
		{
			name:           "Trusted loopback remoteAddr with X-Forwarded-For",
			remoteAddr:     "127.0.0.1:12345",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "203.0.113.195",
		},
		{
			name:           "Trusted private remoteAddr with X-Forwarded-For",
			remoteAddr:     "192.168.1.50:54321",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "203.0.113.195",
		},
		{
			name:           "Untrusted public remoteAddr with X-Forwarded-For (spoofing attempt)",
			remoteAddr:     "198.51.100.2:12345",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "198.51.100.2",
		},
		{
			name:           "Untrusted public remoteAddr without X-Forwarded-For",
			remoteAddr:     "198.51.100.2:12345",
			xForwardedFor:  "",
			expectedResult: "198.51.100.2",
		},
		{
			name:           "IPv6 loopback remoteAddr with X-Forwarded-For",
			remoteAddr:     "[::1]:12345",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "203.0.113.195",
		},
		{
			name:           "IPv6 public remoteAddr with X-Forwarded-For (spoofing attempt)",
			remoteAddr:     "[2001:db8::1]:12345",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "2001:db8::1",
		},
		{
			name:           "No port in remoteAddr loopback with X-Forwarded-For",
			remoteAddr:     "127.0.0.1",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "203.0.113.195",
		},
		{
			name:           "No port in remoteAddr public with X-Forwarded-For",
			remoteAddr:     "198.51.100.2",
			xForwardedFor:  "203.0.113.195",
			expectedResult: "198.51.100.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			result := ExtractRawIP(req)
			if result != tt.expectedResult {
				t.Errorf("expected %q, got %q", tt.expectedResult, result)
			}
		})
	}
}
