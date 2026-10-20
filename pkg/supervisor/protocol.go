package supervisor

import (
	"encoding/json"
)

// Request is a supervisor RPC from the admin process.
type Request struct {
	Op      string          `json:"op"`
	Session string          `json:"session"`
	App     string          `json:"app,omitempty"`
	DBID    string          `json:"db_id,omitempty"`
	SQL     string          `json:"sql,omitempty"`
	Write   bool            `json:"write,omitempty"`
	LogKind string          `json:"log_kind,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

// Response is the RPC result.
type Response struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func encodeResponse(w interface{}) json.RawMessage {
	b, _ := json.Marshal(w)
	return b
}
