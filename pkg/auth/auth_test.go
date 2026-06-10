package auth

import (
	"encoding/base64"
	"net/http"
	"os"
	"testing"
)

func TestAuthenticator(t *testing.T) {
	// Setup env
	os.Setenv("PROXY_USER", "admin")
	os.Setenv("PROXY_PASS", "secret")
	defer os.Unsetenv("PROXY_USER")
	defer os.Unsetenv("PROXY_PASS")

	auth := NewAuthenticator()

	tests := []struct {
		name       string
		authHeader string
		wantErr    bool
	}{
		{
			name:       "missing header",
			authHeader: "",
			wantErr:    true,
		},
		{
			name:       "invalid header format",
			authHeader: "Bearer token123",
			wantErr:    true,
		},
		{
			name:       "invalid base64",
			authHeader: "Basic @@@",
			wantErr:    true,
		},
		{
			name:       "wrong credentials",
			authHeader: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:wrong")),
			wantErr:    true,
		},
		{
			name:       "correct credentials",
			authHeader: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret")),
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			if tt.authHeader != "" {
				req.Header.Set("Proxy-Authorization", tt.authHeader)
			}
			err := auth.CheckAuth(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckAuth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthenticator_EmptyEnv(t *testing.T) {
	os.Unsetenv("PROXY_USER")
	os.Unsetenv("PROXY_PASS")

	auth := NewAuthenticator()
	if auth != nil {
		t.Errorf("Expected nil authenticator with empty env, got: %v", auth)
	}
}
