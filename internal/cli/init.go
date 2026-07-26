package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/scaffold"
)

// newInitCmd builds `mtt init`: write the starter .mtt/config.yaml from a
// built-in name, a local file path, or an https URL.
func newInitCmd() *cobra.Command {
	var (
		tmpl         string
		force        bool
		name         string
		autoYes      bool
		noAgentHooks bool
		noAgentDocs  bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project (.mtt/config.yaml)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			base, err := baseDir(cmd)
			if err != nil {
				return err
			}
			projectName := name
			if projectName == "" {
				projectName = filepath.Base(base)
			}
			kind, value, err := yaml.ClassifyTemplate(tmpl)
			if err != nil {
				return err
			}
			var source string
			switch kind {
			case yaml.SourceBuiltin:
				if err := yaml.Init(base, value, projectName, force); err != nil {
					return err
				}
				source = "builtin:" + value
			case yaml.SourceFile:
				data, rerr := os.ReadFile(value) // #nosec G304 -- the user explicitly names the template file
				if rerr != nil {
					return templateReadError(value, rerr)
				}
				if name != "" {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "note: --name is ignored for an external template (%s)\n", value)
				}
				if err := yaml.InstallConfig(base, data, force); err != nil {
					return err
				}
				source = "file:" + value
				printExternalNotice(cmd, value)
			case yaml.SourceURL:
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "install template from %s? [y/N] ", value)
				ok, cerr := confirmRemote(cmd.InOrStdin(), stdinIsTTY(), autoYes)
				if cerr != nil {
					return cerr
				}
				if !ok {
					_, werr := fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return werr
				}
				data, ferr := yaml.NewFetcher().Fetch(value)
				if ferr != nil {
					return ferr
				}
				if name != "" {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "note: --name is ignored for an external template (%s)\n", value)
				}
				if err := yaml.InstallConfig(base, data, force); err != nil {
					return err
				}
				source = "url:" + value
				printExternalNotice(cmd, value)
			}
			// Config (the switch above) is init's primary job and has succeeded.
			// Scaffold agent hooks then docs by default (opt-out flags); a scaffold
			// failure is reported AFTER config-success and exits 1 — config kept (R3).
			var hookResults, docResults []scaffold.Result
			if !noAgentHooks {
				var serr error
				hookResults, serr = scaffold.Run(base, scaffold.HookTargets())
				if serr != nil {
					// hooks failed: config-success only (nothing scaffolded landed).
					return reportInitConfigThen(cmd, tmpl, nil, serr)
				}
			}
			if !noAgentDocs {
				var derr error
				docResults, derr = scaffold.Run(base, scaffold.DocTargets())
				if derr != nil {
					// docs failed: config-success + every line that DID land (hooks +
					// the doc targets processed before the failure — Run is write-as-you-go).
					return reportInitConfigThen(cmd, tmpl, append(hookResults, docResults...), derr)
				}
			}
			if jsonFlag(cmd) {
				absBase, err := filepath.Abs(base)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), initJSON{
					Path:     filepath.Join(absBase, ".mtt", "config.yaml"),
					Template: tmpl, Source: source, Name: projectName, Created: true,
					Hooks: toScaffoldJSON(hookResults), Docs: toScaffoldJSON(docResults),
				})
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl); err != nil {
				return err
			}
			if err := reportScaffold(cmd, hookResults, hookScaffoldNote); err != nil {
				return err
			}
			return reportScaffold(cmd, docResults, docScaffoldNote)
		},
	}
	cmd.Flags().StringVar(&tmpl, "template", "default", "starter template: default | a file path | an https URL")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config")
	cmd.Flags().StringVar(&name, "name", "", "project name (default: current directory name)")
	cmd.Flags().BoolVar(&autoYes, "yes", false, "skip the confirmation prompt for a remote --template URL")
	cmd.Flags().BoolVar(&noAgentHooks, "no-agent-hooks", false, "skip scaffolding agent settings + hooks (.claude/settings.json)")
	cmd.Flags().BoolVar(&noAgentDocs, "no-agent-docs", false, "skip scaffolding agent docs (AGENTS.md/CLAUDE.md/GEMINI.md)")
	return cmd
}

// templateReadError wraps a failed --template file read, hinting at https:// when
// the arg looks like a scheme-less URL (a dotted host segment before the first /,
// but not a ./ or ../ relative path).
func templateReadError(path string, err error) error {
	if i := strings.IndexByte(path, '/'); i > 0 {
		head := path[:i]
		if head != "." && head != ".." && strings.ContainsRune(head, '.') {
			return fmt.Errorf("read template %q: %w (did you mean https://%s ?)", path, err, path)
		}
	}
	return fmt.Errorf("read template %q: %w", path, err)
}

// reportInitConfigThen prints the config-success line then the per-target human
// lines of what DID land (human mode), and returns the scaffold error → exit 1;
// config is written and kept, no partial JSON (R3). landed is nil on a hooks
// failure (nothing scaffolded), or the hook results on a docs failure.
func reportInitConfigThen(cmd *cobra.Command, tmpl string, landed []scaffold.Result, serr error) error {
	if !jsonFlag(cmd) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl)
		for _, r := range landed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", r.Name, r.Action, r.Path)
		}
	}
	return serr
}

// printExternalNotice is the loud SEC2 review notice for an external install.
func printExternalNotice(cmd *cobra.Command, source string) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
		"\n⚠  installed .mtt/config.yaml from %s\n"+
			"   this config's gate/post commands run via `sh -c` on your transitions.\n"+
			"   review .mtt/config.yaml before your first `mtt` move.\n", source)
}
