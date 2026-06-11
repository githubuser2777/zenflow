package proxy

import (
	"testing"
)

func BenchmarkScrubCookies_Allocations(b *testing.B) {
	cookies := []string{"_ga=12345; session_id=abcdef; _gid=98765; user_pref=darkmode"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scrubCookies(cookies)
	}
}

func BenchmarkScrubCookies_NoAlloc(b *testing.B) {
	cookies := []string{"session_id=abcdef; user_pref=darkmode"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scrubCookies(cookies)
	}
}
