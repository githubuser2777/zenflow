package filter

import (
	"testing"
)

func BenchmarkNormalizeDomain_Allocations(b *testing.B) {
	domain := "[Some.SubDomain.Example.Com.]"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeDomain(domain)
	}
}

func BenchmarkNormalizeDomain_NoAlloc(b *testing.B) {
	domain := "some.subdomain.example.com"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeDomain(domain)
	}
}
