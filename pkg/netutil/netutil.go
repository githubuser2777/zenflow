package netutil

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ExtractRawIP extracts the raw IP string from http.Request.
func ExtractRawIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	cleanHost := host
	if strings.HasPrefix(cleanHost, "[") && strings.HasSuffix(cleanHost, "]") {
		cleanHost = cleanHost[1 : len(cleanHost)-1]
	}

	// Only trust X-Forwarded-For if the request comes from a private/loopback IP
	var isTrusted bool
	if addr, err := netip.ParseAddr(cleanHost); err == nil {
		if addr.IsPrivate() || addr.IsLoopback() {
			isTrusted = true
		}
	}

	if isTrusted {
		ipStr := r.Header.Get("X-Forwarded-For")
		if ipStr != "" {
			if firstIP, _, ok := strings.Cut(ipStr, ","); ok {
				return strings.TrimSpace(firstIP)
			}
			return strings.TrimSpace(ipStr)
		}
	}

	return host
}

// CanonicalizeIP canonicalizes the raw IP.
func CanonicalizeIP(rawIP string) string {
	ipStrClean := rawIP
	isBracketed := false
	if strings.HasPrefix(ipStrClean, "[") && strings.HasSuffix(ipStrClean, "]") {
		ipStrClean = ipStrClean[1 : len(ipStrClean)-1]
		isBracketed = true
	}

	addr, err := netip.ParseAddr(ipStrClean)
	if err != nil {
		return rawIP
	}

	if addr.IsLoopback() && addr.Is6() {
		return "127.0.0.1"
	}

	if addr.Is4() {
		return ipStrClean
	}

	if addr.Is4In6() {
		return addr.Unmap().String()
	}

	canonical6 := addr.String()
	if isBracketed && ipStrClean == canonical6 {
		return rawIP
	}

	return "[" + canonical6 + "]"
}

// ExtractIP extracts and canonicalizes the IP from http.Request.
func ExtractIP(r *http.Request) string {
	return CanonicalizeIP(ExtractRawIP(r))
}
