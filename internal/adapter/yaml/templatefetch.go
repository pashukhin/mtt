package yaml

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	fetchTimeout = 30 * time.Second
	fetchCap     = 1 << 20 // 1 MiB
)

// httpsOnlyRedirect refuses a redirect whose next hop is not https (the real
// http.Client redirect loop invokes this — unit-testable as a pure predicate).
func httpsOnlyRedirect(req *http.Request, _ []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-https %q", req.URL)
	}
	return nil
}

// Fetcher downloads a template over https. The transport is injectable so tests
// run without a socket (a fake RoundTripper), while the real client's redirect
// loop still enforces httpsOnlyRedirect.
type Fetcher struct{ client *http.Client }

// NewFetcher returns a Fetcher over a real https client (30s timeout, https-only
// redirects).
func NewFetcher() *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: fetchTimeout, CheckRedirect: httpsOnlyRedirect}}
}

func newFetcherWithTransport(rt http.RoundTripper) *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: fetchTimeout, CheckRedirect: httpsOnlyRedirect, Transport: rt}}
}

// Fetch GETs url (https only), erroring on a non-200 status and enforcing the
// 1 MiB size cap (LimitReader(cap+1) so an over-cap body errors instead of
// silently truncating).
func (f *Fetcher) Fetch(url string) ([]byte, error) {
	// Self-enforce https even though ClassifyTemplate is the sole caller and only
	// yields https URLs — an exported fetcher must not trust its input scheme.
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("fetch %s: only https is supported", url)
	}
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, fetchCap+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if len(data) > fetchCap {
		return nil, fmt.Errorf("fetch %s: template too large (> %d bytes)", url, fetchCap)
	}
	return data, nil
}
