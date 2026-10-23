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
	"github.com/irgipaulius/cogs/pkg/state"
)

type dnsRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

func zoneNameFromHost(fqdn string) string {
	parts := strings.SplitN(fqdn, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fqdn
}

func findZoneID(ctx context.Context, c *cfclient.Client, zoneName string) (string, error) {
	apiURL := "https://api.cloudflare.com/client/v4/zones?name=" + url.QueryEscape(zoneName)
	out, err := c.Do(ctx, cftoken.ScopeDNS, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	var list struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &list.Result); err != nil {
		return "", err
	}
	if len(list.Result) == 0 {
		return "", fmt.Errorf("zone %q not found", zoneName)
	}
	return list.Result[0].ID, nil
}

func listDNSRecords(ctx context.Context, c *cfclient.Client, zoneID, fqdn string) ([]dnsRecord, error) {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s", zoneID, url.QueryEscape(fqdn))
	out, err := c.Do(ctx, cftoken.ScopeDNS, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	var list struct {
		Result []dnsRecord `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &list.Result); err != nil {
		return nil, err
	}
	return list.Result, nil
}

// EnsureCNAMERecord creates or updates a proxied CNAME; returns record ID.
func EnsureCNAMERecord(ctx context.Context, c *cfclient.Client, zoneID, fqdn, target string) (recordID string, err error) {
	target = strings.TrimSuffix(target, ".")
	records, err := listDNSRecords(ctx, c, zoneID, fqdn)
	if err != nil {
		return "", err
	}
	for _, rec := range records {
		if strings.EqualFold(rec.Type, "CNAME") && strings.EqualFold(strings.TrimSuffix(rec.Content, "."), target) && rec.Proxied {
			return rec.ID, nil
		}
		if rec.Type == "A" || rec.Type == "AAAA" || rec.Type == "CNAME" {
			_ = deleteDNSRecord(ctx, c, zoneID, rec.ID)
		}
	}
	body := map[string]any{
		"type":    "CNAME",
		"name":    fqdn,
		"content": target,
		"proxied": true,
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	out, err := c.Do(ctx, cftoken.ScopeDNS, http.MethodPost, apiURL, body)
	if err != nil {
		return "", err
	}
	var created struct {
		Result dnsRecord `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &created.Result); err != nil {
		return "", err
	}
	return created.Result.ID, nil
}

func ensureTunnelDNS(ctx context.Context, cfg Config, st *state.Store, appID, accountID, tunnelID, fqdn, kind string) error {
	zone := zoneNameFromHost(fqdn)
	zoneID, err := findZoneID(ctx, cfg.Client, zone)
	if err != nil {
		return err
	}
	target := tunnelTarget(tunnelID)
	recID, err := EnsureCNAMERecord(ctx, cfg.Client, zoneID, fqdn, target)
	if err != nil {
		return fmt.Errorf("dns %s: %w", fqdn, err)
	}
	dmeta, _ := json.Marshal(map[string]string{"hostname": fqdn, "zone_id": zoneID, "kind": kind})
	_ = st.RecordOwned(appID, "dns_record", recID, string(dmeta))
	_ = st.ClaimHostname(appID, strings.ToLower(fqdn), kind)
	logf(cfg.Log, "DNS %s → %s", fqdn, target)
	return nil
}

func deleteDNSRecord(ctx context.Context, c *cfclient.Client, zoneID, id string) error {
	if id == "" {
		return nil
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, id)
	_, err := c.Do(ctx, cftoken.ScopeDNS, http.MethodDelete, apiURL, nil)
	return err
}
