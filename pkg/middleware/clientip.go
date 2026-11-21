package middleware

import (
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strings"
)

// Which forwarding headers can be believed depends on what sits in front of
// the server. Trusting the wrong one lets any client invent its own IP and
// walk around rate limits and lockouts, so the choice is explicit:
//
//	vercel      Vercel overwrites X-Vercel-Forwarded-For with the real client IP.
//	cloudflare  Render serves through Cloudflare, which sets CF-Connecting-IP.
//	none        Use the TCP peer address (local development).
//
// TRUSTED_PROXY overrides auto-detection.
var trustedProxy = detectProxy()

func detectProxy() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TRUSTED_PROXY"))) {
	case "vercel", "cloudflare", "none":
		return strings.ToLower(strings.TrimSpace(os.Getenv("TRUSTED_PROXY")))
	}
	if os.Getenv("VERCEL") != "" {
		return "vercel"
	}
	if os.Getenv("RENDER") == "true" || os.Getenv("RENDER_SERVICE_ID") != "" {
		return "cloudflare"
	}
	return "none"
}

// ClientIP returns the best available client address for logging and rate
// limiting.
//
// When the web app's server-side proxy calls this API, the TCP peer is the
// proxy rather than the visitor. The proxy therefore passes the visitor's IP in
// X-Client-IP, signed by a shared secret in X-Proxy-Secret
// (PROXY_SHARED_SECRET). The IP is only believed when the secret matches.
func ClientIP(r *http.Request) string {
	if secret := os.Getenv("PROXY_SHARED_SECRET"); secret != "" {
		given := r.Header.Get("X-Proxy-Secret")
		if subtle.ConstantTimeCompare([]byte(given), []byte(secret)) == 1 {
			if ip := cleanIP(r.Header.Get("X-Client-IP")); ip != "" {
				return ip
			}
		}
	}

	switch trustedProxy {
	case "vercel":
		if ip := cleanIP(r.Header.Get("X-Vercel-Forwarded-For")); ip != "" {
			return ip
		}
		if ip := cleanIP(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	case "cloudflare":
		if ip := cleanIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// cleanIP returns the first valid IP in a possibly comma-separated header.
func cleanIP(v string) string {
	if i := strings.Index(v, ","); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimSpace(v)
	if net.ParseIP(v) == nil {
		return ""
	}
	return v
}
