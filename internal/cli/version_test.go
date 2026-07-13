package cli

import "testing"

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
