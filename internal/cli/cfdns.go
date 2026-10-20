package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type dnsRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

// ensureCNAMERecord creates or updates a proxied CNAME (deletes conflicting A/AAAA).
func ensureCNAMERecord(token, zoneID, fqdn, target string) error {
	target = strings.TrimSuffix(target, ".")
	records, err := listDNSRecords(token, zoneID, fqdn)
	if err != nil {
		return err
	}
	for _, rec := range records {
		if strings.EqualFold(rec.Type, "CNAME") && strings.EqualFold(strings.TrimSuffix(rec.Content, "."), target) && rec.Proxied {
			fmt.Printf("DNS OK: %s CNAME → %s\n", fqdn, target)
			return nil
		}
		if rec.Type == "A" || rec.Type == "AAAA" || rec.Type == "CNAME" {
			if err := deleteDNSRecord(token, zoneID, rec.ID); err != nil {
				fmt.Printf("  note: could not delete old %s record: %v\n", rec.Type, err)
			}
		}
	}
	body := map[string]any{
		"type":    "CNAME",
		"name":    fqdn,
		"content": target,
		"proxied": true,
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	_, err = cfRequest(token, http.MethodPost, apiURL, body)
	if err != nil {
		return err
	}
	fmt.Printf("DNS set: %s CNAME → %s (proxied)\n", fqdn, target)
	return nil
}

func listDNSRecords(token, zoneID, fqdn string) ([]dnsRecord, error) {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s", zoneID, url.QueryEscape(fqdn))
	out, err := cfRequest(token, http.MethodGet, apiURL, nil)
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

func deleteDNSRecord(token, zoneID, id string) error {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, id)
	_, err := cfRequest(token, http.MethodDelete, apiURL, nil)
	return err
}
