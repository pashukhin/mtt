package github

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDoer struct{ resp map[string]string }

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) {
	body, ok := f.resp[req.URL.String()]
	code := http.StatusOK
	if !ok {
		code = http.StatusNotFound
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestLatestParses(t *testing.T) {
	json := `{"tag_name":"v0.9.0","assets":[
		{"name":"mtt_v0.9.0_linux_amd64","browser_download_url":"https://dl/lin"},
		{"name":"SHA256SUMS","browser_download_url":"https://dl/sums"}]}`
	s := newWithDoer(fakeDoer{resp: map[string]string{latestURL: json}})
	rel, err := s.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v0.9.0" || len(rel.Assets) != 2 || rel.Assets[0].URL != "https://dl/lin" {
		t.Fatalf("parsed: %+v", rel)
	}
}

func TestFetch(t *testing.T) {
	s := newWithDoer(fakeDoer{resp: map[string]string{"https://dl/x": "BYTES"}})
	b, err := s.Fetch(context.Background(), "https://dl/x")
	if err != nil || string(b) != "BYTES" {
		t.Fatalf("fetch: %q err=%v", b, err)
	}
}
