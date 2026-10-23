package cfqueue

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatPullError turns queue HTTP failures into actionable token scope hints.
func FormatPullError(status int, body string) error {
	msg := strings.TrimSpace(body)
	var wrap struct {
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal([]byte(body), &wrap)
	if len(wrap.Errors) > 0 {
		msg = wrap.Errors[0].Message
	}
	low := strings.ToLower(msg)
	if status == 401 || strings.Contains(low, "authentication") {
		return fmt.Errorf("queue pull %d: %s — API token needs Account → Queues → Read and Messages → Consume", status, msg)
	}
	if status == 403 || strings.Contains(low, "forbidden") {
		return fmt.Errorf("queue pull %d: %s — grant Cloudflare Queues Read + Messages Consume on the queue", status, msg)
	}
	return fmt.Errorf("queue pull %d: %s", status, msg)
}
