package proxy

import (
	"testing"
	"net/http"
	"net/url"
)

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"", "", true},
		{"a", "", true},
		{"", "a", false},
		{"cache-control: no-cache", "no-cache", true},
		{"cache-control: NO-CACHE", "no-cache", true},
		{"cache-control: private", "private", true},
		{"cache-control: PRIVATE", "private", true},
		{"hello world", "WORLD", true},
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "lo w", true},
		{"hello world", "l w", false},
		{"HELLO WORLD", "world", true},
	}

	for _, tt := range tests {
		if got := containsIgnoreCase(tt.s, tt.substr); got != tt.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v; want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

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
