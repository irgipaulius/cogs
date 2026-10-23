package cfclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/wranglercli"
)

// Client is the single Cloudflare API entry for CoGS.
type Client struct {
	WSRoot string
	OAuth  func() (string, error)
}

// Token resolves a bearer for the given scope.
func (c *Client) Token(ctx context.Context, scope cftoken.Scope) (string, error) {
	r := cftoken.Resolver{WSRoot: c.WSRoot, OAuth: c.OAuth}
	return r.For(ctx, scope)
}

// AccountID returns the active Cloudflare account (env, token API, or wrangler whoami).
func (c *Client) AccountID(ctx context.Context) (string, error) {
	if id := strings.TrimSpace(os.Getenv("COGS_CF_ACCOUNT_ID")); id != "" {
		return id, nil
	}
	if c.WSRoot != "" {
		_ = os.Setenv("COGS_WORKSPACE_ROOT", c.WSRoot)
	}
	if c.WSRoot != "" {
		_ = os.Setenv("COGS_WORKSPACE_ROOT", c.WSRoot)
	}
	if st, err := wranglercli.Status(ctx); err == nil && strings.TrimSpace(st.AccountID) != "" {
		return st.AccountID, nil
	}
	tok, err := c.Token(ctx, cftoken.ScopeAPI)
	if err != nil {
		return "", fmt.Errorf("cloudflare account: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/accounts", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool `json:"success"`
		Result  []struct {
			ID string `json:"id"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("accounts parse: %s", strings.TrimSpace(string(raw)))
	}
	if !out.Success || len(out.Result) == 0 {
		msg := "no Cloudflare account for token"
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return "", fmt.Errorf("%s (set COGS_CF_ACCOUNT_ID or wrangler login)", msg)
	}
	return out.Result[0].ID, nil
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
	tok, err := c.Token(ctx, scope)
	if err != nil {
		return Response{}, err
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
