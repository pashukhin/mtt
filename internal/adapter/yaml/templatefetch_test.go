package yaml

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fetchResp(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestFetchSuccess(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return fetchResp(200, "version: 1\n"), nil
	}))
	b, err := f.Fetch("https://h/x.yaml")
	if err != nil || string(b) != "version: 1\n" {
		t.Fatalf("fetch: %q %v", b, err)
	}
}

func TestFetchNon200(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return fetchResp(404, "nope"), nil
	}))
	if _, err := f.Fetch("https://h/x.yaml"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("want HTTP 404 error, got %v", err)
	}
}

func TestFetchOverCap(t *testing.T) {
	big := strings.Repeat("a", (1<<20)+10) // > 1 MiB
	f := newFetcherWithTransport(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return fetchResp(200, big), nil
	}))
	if _, err := f.Fetch("https://h/x.yaml"); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want too-large error, got %v", err)
	}
}

func TestHTTPSOnlyRedirect(t *testing.T) {
	httpReq, _ := http.NewRequest("GET", "http://evil/x", nil)
	if err := httpsOnlyRedirect(httpReq, nil); err == nil {
		t.Fatal("redirect to http must be refused")
	}
	httpsReq, _ := http.NewRequest("GET", "https://ok/x", nil)
	if err := httpsOnlyRedirect(httpsReq, nil); err != nil {
		t.Fatalf("https redirect must be allowed: %v", err)
	}
}

// The seam-wiring test: a 302 → http hop must be refused by the REAL client's
// CheckRedirect (a fake transport alone can't prove this — only a real
// http.Client runs CheckRedirect). The error must NAME the scheme — otherwise the
// test would also pass on the client's default 10-redirect-loop limit, i.e. even
// if httpsOnlyRedirect were removed/broken (a false-confidence test).
func TestFetchRefusesRedirectToHTTP(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("Location", "http://evil/x.yaml")
		return &http.Response{StatusCode: 302, Body: io.NopCloser(strings.NewReader("")), Header: h}, nil
	}))
	_, err := f.Fetch("https://h/x.yaml")
	if err == nil || !strings.Contains(err.Error(), "non-https") {
		t.Fatalf("a redirect to http must be refused by CheckRedirect (error naming the scheme), got %v", err)
	}
}

// The exported fetcher self-enforces https on the initial request (not just
// redirects) — it must not trust its input scheme.
func TestFetchRejectsNonHTTPS(t *testing.T) {
	f := newFetcherWithTransport(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("transport must not be reached for a non-https URL")
		return nil, nil
	}))
	if _, err := f.Fetch("http://h/x.yaml"); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("non-https initial URL must be rejected before any request, got %v", err)
	}
}
