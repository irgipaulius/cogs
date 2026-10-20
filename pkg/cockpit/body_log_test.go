package cockpit

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCaptureRequestBodyJSON(t *testing.T) {
	payload := `{"title":"buy milk"}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/todos", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	got := captureRequestBody(req)
	if !strings.Contains(got, "buy milk") {
		t.Fatalf("got %q", got)
	}
	b, _ := io.ReadAll(req.Body)
	if string(b) != payload {
		t.Fatalf("handler body mutated")
	}
}

func TestCaptureRequestBodyMultipart(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://x/upload", bytes.NewReader([]byte("x")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	if got := captureRequestBody(req); got != "[multipart omitted]" {
		t.Fatalf("got %q", got)
	}
}
