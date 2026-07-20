package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/github"
	"github.com/pashukhin/mtt/internal/adapter/installer"
	"github.com/pashukhin/mtt/internal/core"
)

// selfUpdateJSON is the pinned --json shape.
type selfUpdateJSON struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	Updated         bool   `json:"updated"`
	Via             string `json:"via"`
	Asset           string `json:"asset,omitempty"`
	Path            string `json:"path,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Error           string `json:"error,omitempty"`
}

// toSelfUpdateJSON builds the view from a plan (+ result if applied). err, when
// non-nil, populates the error field (the object still renders on a failure path).
func toSelfUpdateJSON(p core.Plan, r core.Result, applied bool, err error) selfUpdateJSON {
	j := selfUpdateJSON{
		Current:         p.Current,
		Latest:          p.Latest,
		UpdateAvailable: p.State == core.UpdateAvailable,
		Updated:         applied,
		Via:             string(p.Via),
		Reason:          p.Reason,
	}
	// asset/path describe what was actually fetched/installed — only on apply (D8:
	// under --check-only nothing is fetched, so both stay empty).
	if applied {
		j.Via = string(r.Via)
		j.Asset = p.AssetName
		j.Path = r.Path
	}
	if err != nil {
		j.Error = err.Error()
	}
	return j
}

// newSelfUpdateCmd builds `mtt self-update`.
func newSelfUpdateCmd() *cobra.Command {
	var checkOnly, force bool
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update the installed mtt binary to the latest release",
		Long: "Download the latest published release asset, verify its SHA-256, and atomically\n" +
			"replace the running binary. Falls back to `go install` when no verifiable asset\n" +
			"matches this platform. --check-only reports availability without writing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current := resolveVersion()

			// Hermetic short-circuit: an unorderable current with neither --force nor
			// --check-only always refuses — decide it BEFORE any network call.
			if !force && !checkOnly && !core.Orderable(current) {
				refusal := fmt.Errorf("cannot determine the current version (%q); re-run with --force to update anyway", current)
				if jsonFlag(cmd) {
					_ = writeJSON(cmd.OutOrStdout(), toSelfUpdateJSON(core.Plan{Current: current}, core.Result{}, false, refusal))
				}
				return refusal
			}

			_, goErr := exec.LookPath("go")
			src := github.New()
			updater := core.NewSelfUpdater()

			plan, err := updater.Prepare(cmd.Context(), current, runtime.GOOS, runtime.GOARCH, goErr == nil, force, src)
			if err != nil {
				if jsonFlag(cmd) {
					_ = writeJSON(cmd.OutOrStdout(), toSelfUpdateJSON(core.Plan{Current: current}, core.Result{}, false, err))
				}
				return err
			}

			if checkOnly { // reports the plan; nothing fetched, so no target needed
				return renderSelfUpdate(cmd, plan, core.Result{}, false, "", nil)
			}

			// Apply path: resolve the running binary to replace.
			target, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate running binary: %w", err)
			}
			if resolved, err := filepath.EvalSymlinks(target); err == nil {
				target = resolved
			}

			switch plan.State {
			case core.NoUpdate:
				return renderSelfUpdate(cmd, plan, core.Result{}, false, target, nil)
			case core.Undetermined:
				// Defensive: unreachable here — an unorderable current without --force
				// short-circuits before Prepare, and --force never yields Undetermined.
				return renderSelfUpdate(cmd, plan, core.Result{}, false, target, errors.New(plan.Reason))
			default: // UpdateAvailable
				if plan.Via == core.ViaNone {
					return renderSelfUpdate(cmd, plan, core.Result{}, false, target, errors.New(plan.Reason))
				}
				res, err := updater.Apply(cmd.Context(), plan, src, installer.NewReplacer(), installer.NewGoInstaller(), target)
				if err != nil {
					return renderSelfUpdate(cmd, plan, core.Result{}, false, target, err)
				}
				return renderSelfUpdate(cmd, plan, res, true, target, nil)
			}
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check-only", false, "report whether an update is available; write nothing")
	cmd.Flags().BoolVar(&force, "force", false, "update even from a dev build or when already on the latest")
	return cmd
}

// renderSelfUpdate prints text (or JSON) for a plan/result. runningPath is the
// resolved path of the currently-running binary (used only for the go-install
// "different location" note — D7). A non-nil err makes it return that err
// (→ exit 1) after emitting the JSON object / a stderr message.
func renderSelfUpdate(cmd *cobra.Command, p core.Plan, r core.Result, applied bool, runningPath string, err error) error {
	if jsonFlag(cmd) {
		if werr := writeJSON(cmd.OutOrStdout(), toSelfUpdateJSON(p, r, applied, err)); werr != nil {
			return werr
		}
		return err
	}
	if err != nil {
		return err // Execute() prints "error: <msg>" to stderr, exit 1
	}
	var b strings.Builder
	switch {
	case applied:
		if r.Via == core.ViaGoInstall {
			fmt.Fprintf(&b, "updated to %s via go install → %s\n", r.Tag, r.Path)
			if r.Path != "" && r.Path != runningPath { // D7: note only when it landed elsewhere
				fmt.Fprintf(&b, "note: the updated binary is at %s, not the running %s (ensure it is the mtt on your PATH)\n", r.Path, runningPath)
			}
		} else {
			fmt.Fprintf(&b, "updated %s → %s\n", p.Current, r.Tag)
		}
	case p.State == core.NoUpdate:
		fmt.Fprintf(&b, "already up to date (%s)\n", p.Current)
	case p.State == core.UpdateAvailable && p.Via == core.ViaNone:
		fmt.Fprintf(&b, "update available (%s), but %s\n", p.Latest, p.Reason)
	case p.State == core.UpdateAvailable:
		fmt.Fprintf(&b, "update available: %s → %s (via %s)\n", p.Current, p.Latest, p.Via)
	case p.State == core.Undetermined:
		fmt.Fprintf(&b, "%s\n", p.Reason)
	}
	_, werr := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return werr
}
