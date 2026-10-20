package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HTTPD1 implements D1Client via Cloudflare REST API.
type HTTPD1 struct {
	AccountID  string
	DatabaseID string
	Token      string
	HTTPClient *http.Client
}

type d1QueryBody struct {
	SQL    string   `json:"sql,omitempty"`
	Params []string `json:"params,omitempty"`
	Batch  []d1QueryBody `json:"batch,omitempty"`
}

type d1Response struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		Results []map[string]any `json:"results"`
	} `json:"result"`
}

// Query runs a read query and returns stringified rows (column order not guaranteed; tests use fakes).
func (c *HTTPD1) Query(ctx context.Context, sql string, params ...any) ([][]string, error) {
	pstr := make([]string, len(params))
	for i, p := range params {
		pstr[i] = fmt.Sprint(p)
	}
	body, err := c.post(ctx, d1QueryBody{SQL: sql, Params: pstr})
	if err != nil {
		return nil, err
	}
	var out [][]string
	for _, res := range body.Result {
		for _, row := range res.Results {
			var line []string
			for _, v := range row {
				line = append(line, fmt.Sprint(v))
			}
			out = append(out, line)
		}
	}
	return out, nil
}

// ExecBatch runs a batch of statements in one HTTP request.
func (c *HTTPD1) ExecBatch(ctx context.Context, statements []Statement) error {
	if len(statements) == 0 {
		return nil
	}
	batch := make([]d1QueryBody, len(statements))
	for i, s := range statements {
		batch[i] = d1QueryBody{SQL: s.SQL, Params: s.Params}
	}
	_, err := c.post(ctx, d1QueryBody{Batch: batch})
	return err
}

func (c *HTTPD1) post(ctx context.Context, payload d1QueryBody) (*d1Response, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query", c.AccountID, c.DatabaseID)
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var dr d1Response
	if err := json.Unmarshal(raw, &dr); err != nil {
		return nil, err
	}
	if !dr.Success {
		msg := "d1 query failed"
		if len(dr.Errors) > 0 {
			msg = dr.Errors[0].Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return &dr, nil
}
