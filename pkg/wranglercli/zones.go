package wranglercli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
)

// Zone is a Cloudflare DNS zone on the logged-in account.
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type zonesResponse struct {
	Success bool   `json:"success"`
	Result  []Zone `json:"result"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ListZones returns zones for the API token account (empty if not logged in).
func ListZones(ctx context.Context) ([]Zone, error) {
	wsRoot := strings.TrimSpace(os.Getenv("COGS_WORKSPACE_ROOT"))
	res := cftoken.Resolver{WSRoot: wsRoot, OAuth: cftoken.WranglerOAuthToken}
	tok, err := res.For(ctx, cftoken.ScopeAPI)
	if err != nil {
		return nil, nil
	}
	accountID := strings.TrimSpace(os.Getenv("COGS_CF_ACCOUNT_ID"))
	if accountID == "" {
		st, _ := Status(ctx)
		accountID = st.AccountID
	}
	if accountID == "" {
		return nil, nil
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?account.id=%s&per_page=50", accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var zr zonesResponse
	if err := json.Unmarshal(body, &zr); err != nil {
		return nil, err
	}
	if !zr.Success {
		msg := "zones API failed"
		if len(zr.Errors) > 0 {
			msg = zr.Errors[0].Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return zr.Result, nil
}
