// Package github implements mtt.ReleaseSource over the GitHub Releases HTTP API.
// The HTTP client is injectable (an httpDoer) so tests run without a socket. Per-
// operation context deadlines bound the API probe and the asset download
// separately; the default client sets no global Timeout and does not override
// redirect/proxy behavior (so browser_download_url redirects and HTTP(S)_PROXY
// keep working).
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

const (
	latestURL       = "https://api.github.com/repos/pashukhin/mtt/releases/latest"
	apiTimeout      = 15 * time.Second
	downloadTimeout = 3 * time.Minute
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Source is the GitHub-backed ReleaseSource.
type Source struct{ doer httpDoer }

// New returns a Source over the default HTTP client (no global Timeout — the
// per-operation context governs; redirects/proxy left at their defaults).
func New() *Source { return &Source{doer: &http.Client{}} }

func newWithDoer(d httpDoer) *Source { return &Source{doer: d} }

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest returns the newest release.
func (s *Source) Latest(ctx context.Context) (mtt.Release, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return mtt.Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.doer.Do(req)
	if err != nil {
		return mtt.Release{}, fmt.Errorf("GET latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return mtt.Release{}, fmt.Errorf("GET latest release: unexpected status %d", resp.StatusCode)
	}
	var gr ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return mtt.Release{}, fmt.Errorf("decode release: %w", err)
	}
	rel := mtt.Release{Tag: gr.TagName}
	for _, a := range gr.Assets {
		rel.Assets = append(rel.Assets, mtt.ReleaseAsset{Name: a.Name, URL: a.URL})
	}
	return rel, nil
}

// Fetch downloads the bytes at url.
func (s *Source) Fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: unexpected status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
