package proxy

import (
	"net"
	"net/http"

	"zenflow/pkg/auth"
	"zenflow/pkg/filter"
	"zenflow/pkg/logger"
	"zenflow/pkg/netutil"
	"zenflow/pkg/ratelimit"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func RateLimitMiddleware(limiter *ratelimit.TokenBucketLimiter, rateLimitStr string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := netutil.ExtractIP(r)
			if err := limiter.CheckRateLimit(ip); err != nil {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			w.Header().Set("X-RateLimit-Limit", rateLimitStr)
			next.ServeHTTP(w, r)
		})
	}
}

func AuthMiddleware(authenticator auth.Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authenticator != nil {
				if err := authenticator.CheckAuth(r); err != nil {
					w.Header().Set("Proxy-Authenticate", `Basic realm="Proxy"`)
					http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func BlockerMiddleware(domainBlocker, malwareBlocker *filter.Blocker) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if host == "" {
				h, _, err := net.SplitHostPort(r.URL.Host)
				if err != nil {
					host = r.URL.Host
				} else {
					host = h
				}
			}
			if domainBlocker != nil {
				if blocked, reason := domainBlocker.IsBlocked(host); blocked {
					logger.LogBlock(r.Method, host, r.URL.String(), reason)
					http.Error(w, "Blocked by proxy filter", http.StatusForbidden)
					return
				}
			}
			if malwareBlocker != nil {
				if blocked, reason := malwareBlocker.IsBlocked(host); blocked {
					logger.LogBlock(r.Method, host, r.URL.String(), "Malware: "+reason)
					http.Error(w, "Blocked by malware filter", http.StatusForbidden)
					return
				}
			}
			logger.LogAllow(r.Method, host, r.URL.String())
			next.ServeHTTP(w, r)
		})
	}
}
