package ratelimit

import (
	"testing"
)

func BenchmarkCanonicalizeIP_Allocations(b *testing.B) {
	ip := "[2001:db8::1]"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = canonicalizeIP(ip)
	}
}

func BenchmarkCanonicalizeIP_NoAlloc(b *testing.B) {
	ip := "192.168.1.1"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = canonicalizeIP(ip)
	}
}
