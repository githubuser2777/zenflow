package proxy

import (
	"testing"
	"net/http"
	"net/url"
)



func TestIsStaticAsset(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/image.png", true},
		{"/image.PNG", true},
		{"/image.jpg", true},
		{"/image.jpeg", true},
		{"/style.css", true},
		{"/script.js", true},
		{"/favicon.ico", true},
		{"/image.gif", true},
		{"/page.html", false},
		{"/", false},
		{"/image.png/something", false},
		{"/no-extension", false},
		{"/.png", true},
	}

	for _, tt := range tests {
		req := &http.Request{URL: &url.URL{Path: tt.path}}
		if got := isStaticAsset(req); got != tt.want {
			t.Errorf("isStaticAsset(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}
}
