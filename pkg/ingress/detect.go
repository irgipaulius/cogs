package ingress

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// ViaCloudflare reports whether the request likely arrived through Cloudflare (Tunnel or proxied HTTP).
func ViaCloudflare(r *http.Request) bool {
	if r.Header.Get("Cf-Connecting-Ip") != "" || r.Header.Get("CF-Connecting-IP") != "" {
		return true
	}
	if r.Header.Get("Cf-Ray") != "" || r.Header.Get("CF-RAY") != "" {
		return true
	}
	if r.Header.Get("Cf-Visitor") != "" {
		return true
	}
	return false
}

// IsLoopbackPeer is true when the TCP client is on loopback (local tunnel connector or dev curl).
func IsLoopbackPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func directSecretOK(r *http.Request, envVar string) bool {
	if envVar == "" {
		return true
	}
	want := strings.TrimSpace(os.Getenv(envVar))
	if want == "" {
		return true
	}
	return r.Header.Get("X-Cogs-Direct-Key") == want
}
