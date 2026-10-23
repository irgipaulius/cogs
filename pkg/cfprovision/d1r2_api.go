package cfprovision

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cftoken"
)

func ensureD1Database(ctx context.Context, c *cfclient.Client, accountID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, err := findD1Database(ctx, c, accountID, name); err == nil {
		return nil
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database", accountID)
	body := map[string]any{"name": name}
	_, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("d1 create %q: %w", name, apiPermissionHint("D1", err))
	}
	return nil
}

func findD1Database(ctx context.Context, c *cfclient.Client, accountID, name string) (string, error) {
	q := url.Values{}
	q.Set("name", name)
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database?%s", accountID, q.Encode())
	out, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	var listed struct {
		Result []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &listed.Result); err != nil {
		return "", err
	}
	for _, db := range listed.Result {
		if db.Name == name {
			return db.UUID, nil
		}
	}
	return "", fmt.Errorf("d1 database %q not found", name)
}

func ensureR2Bucket(ctx context.Context, c *cfclient.Client, accountID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if ok, err := r2BucketExists(ctx, c, accountID, name); err != nil {
		return err
	} else if ok {
		return nil
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets", accountID)
	body := map[string]any{"name": name}
	_, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("r2 bucket create %q: %w", name, apiPermissionHint("R2", err))
	}
	return nil
}

func r2BucketExists(ctx context.Context, c *cfclient.Client, accountID, name string) (bool, error) {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets", accountID)
	out, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, err
	}
	var listed struct {
		Result struct {
			Buckets []struct {
				Name string `json:"name"`
			} `json:"buckets"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &listed.Result); err != nil {
		// Some accounts return a bare array.
		var buckets []struct {
			Name string `json:"name"`
		}
		if err2 := json.Unmarshal(out.Result, &buckets); err2 != nil {
			return false, err
		}
		for _, b := range buckets {
			if b.Name == name {
				return true, nil
			}
		}
		return false, nil
	}
	for _, b := range listed.Result.Buckets {
		if b.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func apiPermissionHint(product string, err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch product {
	case "D1":
		if strings.Contains(msg, "auth") || strings.Contains(msg, "10000") || strings.Contains(msg, "403") {
			return fmt.Errorf("%v — grant Account → D1 → Edit in Cloudflare API token", err)
		}
	case "R2":
		if strings.Contains(msg, "auth") || strings.Contains(msg, "10000") || strings.Contains(msg, "403") {
			return fmt.Errorf("%v — grant Account → R2 → Edit in Cloudflare API token", err)
		}
	case "Tunnel":
		if strings.Contains(msg, "auth") || strings.Contains(msg, "10000") || strings.Contains(msg, "403") {
			return fmt.Errorf("%v — grant Account → Cloudflare Tunnel → Edit in Cloudflare API token", err)
		}
	}
	return err
}
