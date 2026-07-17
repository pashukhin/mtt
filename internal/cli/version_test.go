package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name         string
		ldflags      string
		buildVersion string
		want         string
	}{
		{"ldflags value wins", "v0.9.0", "", "v0.9.0"},
		{"ldflags wins over build info", "v0.9.0", "v1.2.3", "v0.9.0"},
		{"build info when ldflags is dev", "dev", "v1.2.3", "v1.2.3"},
		{"fallback on (devel) build info", "dev", "(devel)", "dev"},
		{"fallback on empty build info", "dev", "", "dev"},
		{"empty ldflags falls through to build info", "", "v1.2.3", "v1.2.3"},
		{"fallback on empty ldflags", "", "", "dev"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolve(tc.ldflags, func() string { return tc.buildVersion })
			if got != tc.want {
				t.Fatalf("resolve(%q, ()->%q) = %q, want %q", tc.ldflags, tc.buildVersion, got, tc.want)
			}
		})
	}
}

func TestVersionJSON(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got.Version != resolveVersion() {
		t.Fatalf("version = %q, want %q", got.Version, resolveVersion())
	}
}

// The cobra --version flag must honor --json too (not just the subcommand), so
// `mtt --version --json` and `mtt version --json` agree.
func TestVersionFlagJSON(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("--version --json not JSON: %v\n%s", err, out.String())
	}
	if got.Version != resolveVersion() {
		t.Fatalf("version = %q, want %q", got.Version, resolveVersion())
	}
}

func TestVersionFlagText(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, want := strings.TrimSpace(out.String()), "mtt version "+resolveVersion(); got != want {
		t.Fatalf("--version = %q, want %q", got, want)
	}
}

// --version is a local root flag, so it must NOT leak onto subcommands.
func TestVersionFlagNotOnSubcommands(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"list", "--version"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("list --version must be an unknown flag; got err=%v out=%q", err, out.String())
	}
}
