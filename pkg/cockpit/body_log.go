package cockpit

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	maxBodyRead    = 16 << 10 // 16 KiB max read for logging path
	maxBodyLogRunes = 600
)

func captureRequestBody(r *http.Request) string {
	if r == nil || r.Body == nil || !isWriteMethod(r.Method) {
		return ""
	}
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/") {
		return "[multipart omitted]"
	}
	if r.ContentLength > maxBodyRead && r.ContentLength > 0 {
		return formatBodySnippet(nil, true, int(r.ContentLength))
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return "[body read error]"
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return ""
	}
	truncated := len(body) > maxBodyRead
	if truncated {
		body = body[:maxBodyRead]
	}
	return formatBodySnippet(body, truncated, len(body))
}

func formatBodySnippet(body []byte, truncated bool, sizeHint int) string {
	if len(body) == 0 && truncated {
		if sizeHint > 0 {
			return "[body " + itoa(sizeHint) + " bytes omitted]"
		}
		return "[body omitted]"
	}
	s := strings.TrimSpace(string(body))
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if !utf8.ValidString(s) {
		return "[binary " + itoa(len(body)) + " bytes]"
	}
	runes := []rune(s)
	if len(runes) > maxBodyLogRunes {
		s = string(runes[:maxBodyLogRunes]) + "…"
		truncated = true
	}
	if truncated {
		s += " [truncated]"
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
