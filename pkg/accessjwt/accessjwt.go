// Package accessjwt validates Cloudflare Access JWT assertions.
package accessjwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Validator fetches team certs and validates Cf-Access-Jwt-Assertion tokens.
type Validator struct {
	TeamDomain string
	Audience   string
	HTTPClient *http.Client

	mu    sync.RWMutex
	keys  map[string]*rsa.PublicKey
	fetch time.Time
	ttl   time.Duration
}

type certsResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Kty string `json:"kty"`
}

// Validate parses and verifies an Access JWT.
func (v *Validator) Validate(ctx context.Context, tokenString string) (*jwt.Token, error) {
	if v.TeamDomain == "" {
		return nil, errors.New("accessjwt: team domain not set")
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	keys := v.keys
	v.mu.RUnlock()

	keyFunc := func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok {
			return nil, errors.New("accessjwt: missing kid")
		}
		k, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("accessjwt: unknown kid %q", kid)
		}
		return k, nil
	}
	token, err := jwt.Parse(tokenString, keyFunc, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("accessjwt: invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("accessjwt: bad claims")
	}
	if v.Audience != "" {
		if aud, _ := claims["aud"].(string); aud != v.Audience {
			return nil, fmt.Errorf("accessjwt: audience mismatch")
		}
	}
	return token, nil
}

func (v *Validator) refresh(ctx context.Context) error {
	v.mu.RLock()
	stale := time.Since(v.fetch) > v.certTTL()
	v.mu.RUnlock()
	if !stale && len(v.keys) > 0 {
		return nil
	}
	url := fmt.Sprintf("https://%s/cdn-cgi/access/certs", v.TeamDomain)
	client := v.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
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
	var cr certsResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(cr.Keys))
	for _, k := range cr.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("accessjwt: no keys in certs response")
	}
	v.mu.Lock()
	v.keys = keys
	v.fetch = time.Now()
	v.mu.Unlock()
	return nil
}

func (v *Validator) certTTL() time.Duration {
	if v.ttl > 0 {
		return v.ttl
	}
	return time.Hour
}

// LoadFixtureKeys sets keys from a JWKS JSON fixture (tests).
func (v *Validator) LoadFixtureKeys(raw []byte) error {
	var cr certsResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, k := range cr.Keys {
		pub, err := rsaFromJWK(k.N, k.E)
		if err != nil {
			return err
		}
		keys[k.Kid] = pub
	}
	v.mu.Lock()
	v.keys = keys
	v.fetch = time.Now()
	v.mu.Unlock()
	return nil
}

func rsaFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	var eInt int
	for _, b := range eb {
		eInt = eInt<<8 + int(b)
	}
	if eInt == 0 {
		eInt = 65537
	}
	n := new(big.Int).SetBytes(nb)
	return &rsa.PublicKey{N: n, E: eInt}, nil
}
