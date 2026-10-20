package linkperf

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingAndBench(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, DefaultOptions())
	s := httptest.NewServer(mux)
	defer s.Close()

	res, err := http.Get(s.URL + PingPath)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}

	res2, err := http.Get(s.URL + BenchPath + "?bytes=4096")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	b, err := io.ReadAll(res2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 4096 {
		t.Fatalf("len=%d", len(b))
	}
}
