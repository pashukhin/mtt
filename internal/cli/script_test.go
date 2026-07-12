package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// mttEnvVars are the environment seams the mtt CLI reads: MTT_DIR overrides
// project discovery; MTT_BY/MTT_ROLE seed history attribution. A value exported
// in the developer's shell would override the tests' own cwd-discovery /
// attribution and deterministically redden the suite — and, worse, `init`
// resolving to $MTT_DIR would scatter a stray `.mtt` into that dir, poisoning
// cwd-discovery for other packages too. The harness scrubs them so the tests
// run against a clean, hermetic environment regardless of the caller's shell (c4).
var mttEnvVars = []string{"MTT_DIR", "MTT_BY", "MTT_ROLE"}

func scrubMttEnv() {
	for _, k := range mttEnvVars {
		os.Unsetenv(k)
	}
}

// inMttCommandSubprocess reports whether argv0 is the re-invoked "mtt" command
// (testscript.Main dispatches a registered command when os.Args[0]'s basename
// matches it — see exe.go). In that subprocess MTT_* must NOT be scrubbed: it
// runs against the per-script env that legitimately carries `env MTT_DIR=…`.
func inMttCommandSubprocess(argv0 string) bool {
	name := filepath.Base(argv0)
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(name, ".exe")
	}
	return name == "mtt"
}

func TestMain(m *testing.M) {
	// Scrub MTT_* only in the harness process (in-process tests + the source of
	// the `.mtt`-into-$MTT_DIR pollution), never in the mtt command subprocess.
	if !inMttCommandSubprocess(os.Args[0]) {
		scrubMttEnv()
	}
	testscript.Main(m, map[string]func(){
		"mtt": func() { os.Exit(Execute()) },
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: "testdata/scripts"})
}

// TestScrubMttEnv guards the harness scrub: an inherited MTT_DIR/MTT_BY/MTT_ROLE
// must be cleared, else the whole cli suite reddens under a developer's exported env.
func TestScrubMttEnv(t *testing.T) {
	for _, k := range mttEnvVars {
		t.Setenv(k, "leaked-"+k)
	}
	scrubMttEnv()
	for _, k := range mttEnvVars {
		if v := os.Getenv(k); v != "" {
			t.Errorf("scrubMttEnv left %s=%q set", k, v)
		}
	}
}

// TestInMttCommandSubprocess pins the gate that keeps the scrub out of the mtt
// command subprocess: the harness binary must scrub, the "mtt" re-invocation must not.
func TestInMttCommandSubprocess(t *testing.T) {
	if inMttCommandSubprocess("/tmp/go-build123/b001/cli.test") {
		t.Error("harness test binary misdetected as the mtt command subprocess")
	}
	if !inMttCommandSubprocess("/tmp/testscript-main456/bin/mtt") {
		t.Error("mtt command subprocess not detected")
	}
}
