// Package session implements Turnstile verification and HMAC session tokens.
package session

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTTL = 15 * time.Minute

// Manager verifies Turnstile and mints HMAC-bound session tokens.
type Manager struct {
	Secret          []byte
	TurnstileSecret string
	TTL             time.Duration
	HTTPClient      *http.Client
}

// TurnstileResponse is the siteverify JSON body.
type TurnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// VerifyTurnstile calls Cloudflare siteverify with the client response token.
func (m *Manager) VerifyTurnstile(ctx context.Context, token, remoteIP string) error {
	if m.TurnstileSecret == "" {
		return errors.New("session: turnstile secret not configured")
	}
	form := url.Values{}
	form.Set("secret", m.TurnstileSecret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := m.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var tr TurnstileResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return err
	}
	if !tr.Success {
		return fmt.Errorf("session: turnstile failed: %v", tr.ErrorCodes)
	}
	return nil
}

// Mint creates a token bound to clientIP until expiry.
func (m *Manager) Mint(clientIP string, now time.Time) (string, error) {
	if len(m.Secret) == 0 {
		return "", errors.New("session: empty secret")
	}
	ttl := m.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	exp := now.Add(ttl).Unix()
	payload := fmt.Sprintf("%d|%s", exp, clientIP)
	mac := hmac.New(sha256.New, m.Secret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Validate checks token integrity, expiry, and IP binding.
func (m *Manager) Validate(token, clientIP string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return errors.New("session: malformed token")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("session: bad payload encoding")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("session: bad sig encoding")
	}
	mac := hmac.New(sha256.New, m.Secret)
	mac.Write(payloadBytes)
	if !hmac.Equal(mac.Sum(nil), sigBytes) {
		return errors.New("session: tampered token")
	}
	payload := string(payloadBytes)
	expStr, ip, ok := strings.Cut(payload, "|")
	if !ok {
		return errors.New("session: bad payload")
	}
	var exp int64
	if _, err := fmt.Sscanf(expStr, "%d", &exp); err != nil {
		return errors.New("session: bad expiry")
	}
	if now.Unix() > exp {
		return errors.New("session: expired token")
	}
	if ip != clientIP {
		return errors.New("session: wrong ip")
	}
	return nil
}

// Middleware requires Authorization: Bearer <token> or Cookie cogs_session.
func (m *Manager) Middleware(clientIPFromRequest func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := bearerToken(r)
			if tok == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ip := clientIPFromRequest(r)
			if err := m.Validate(tok, ip, time.Now()); err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if c, err := r.Cookie("cogs_session"); err == nil {
		return c.Value
	}
	return ""
}
