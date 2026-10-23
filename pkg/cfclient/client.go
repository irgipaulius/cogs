package cfclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
)

// Client is the single Cloudflare API entry for CoGS.
type Client struct {
	WSRoot string
	OAuth  func() (string, error)
	// TestBase rewrites https://api.cloudflare.com to this prefix (tests only).
	TestBase  string
	TestToken string
}

// Token resolves a bearer for the given scope.
func (c *Client) Token(ctx context.Context, scope cftoken.Scope) (string, error) {
	r := cftoken.Resolver{WSRoot: c.WSRoot, OAuth: c.OAuth}
	return r.For(ctx, scope)
}

// Response is a Cloudflare v4 API envelope.
type Response struct {
	Success bool            `json:"success"`
	Errors  []APIError      `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Do performs an authenticated Cloudflare API request.
func (c *Client) Do(ctx context.Context, scope cftoken.Scope, method, url string, body any) (Response, error) {
	if base := strings.TrimSpace(c.TestBase); base != "" {
		url = strings.Replace(url, "https://api.cloudflare.com", base, 1)
	}
	var tok string
	var err error
	if strings.TrimSpace(c.TestToken) != "" {
		tok = c.TestToken
	} else {
		tok, err = c.Token(ctx, scope)
		if err != nil {
			return Response{}, err
		}
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return Response{}, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	var out Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, fmt.Errorf("cloudflare api: %s", string(raw))
	}
	if !out.Success {
		msg := "cloudflare api error"
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}
