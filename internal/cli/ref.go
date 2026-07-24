package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// parseRefArg parses "<kind>:<target>" (split on the FIRST colon — a url has more).
// kind must be note/task/url; comment is rejected (t2); each target is validated by
// its own identity rule. Label is empty (the caller sets it from --label).
func parseRefArg(arg string) (mtt.Ref, error) {
	k, target, found := strings.Cut(arg, ":")
	if !found {
		return mtt.Ref{}, fmt.Errorf("expected <kind>:<target> (example: task:t2), got %q", arg)
	}
	kind := mtt.RefKind(k)
	if kind == mtt.RefComment {
		return mtt.Ref{}, fmt.Errorf("comments arrive in t2; the 'comment' ref kind is not yet supported")
	}
	switch kind {
	case mtt.RefTask:
		if _, err := mtt.NewTaskID(target); err != nil {
			return mtt.Ref{}, fmt.Errorf("invalid task target %q: %w", target, err)
		}
	case mtt.RefNote:
		if _, err := mtt.NewNoteSlug(target); err != nil {
			return mtt.Ref{}, fmt.Errorf("invalid note target %q: %w", target, err)
		}
	case mtt.RefURL:
		u, err := url.Parse(target)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return mtt.Ref{}, fmt.Errorf("invalid url target %q (need scheme and host, e.g. https://example.com/x)", target)
		}
	default:
		return mtt.Ref{}, fmt.Errorf("unknown ref kind %q (want note|task|url)", k)
	}
	return mtt.Ref{Kind: kind, ID: target}, nil
}

// refJSON is one reference in a machine-readable view: kind/id/label + resolution status.
type refJSON struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
	Status string `json:"status"`
}

func toRefJSON(r mtt.Ref, st core.RefStatus) refJSON {
	return refJSON{Kind: string(r.Kind), ID: r.ID, Label: r.Label, Status: string(st)}
}

func refLine(r mtt.Ref, st core.RefStatus) string {
	s := fmt.Sprintf("%s:%s  [%s]", r.Kind, r.ID, st)
	if r.Label != "" {
		s += "  (" + r.Label + ")"
	}
	return s
}

// backlinkJSON is one incoming backlink (carrier kind + id + the forward ref's
// label). Reused by ref list / note ref list / show / note show.
type backlinkJSON struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

func verifiedRefsJSON(refs []mtt.Ref, te func(mtt.TaskID) bool, ne func(mtt.NoteSlug) bool) []refJSON {
	out := make([]refJSON, 0, len(refs))
	for _, r := range refs {
		out = append(out, toRefJSON(r, core.VerifyRef(r, te, ne)))
	}
	return out
}

func toBacklinkJSON(rs []core.Referent) []backlinkJSON {
	out := make([]backlinkJSON, 0, len(rs))
	for _, r := range rs {
		out = append(out, backlinkJSON{Kind: string(r.Carrier), ID: r.ID, Label: r.Label})
	}
	return out
}

func taskExistsFn(tasks []mtt.Task) func(mtt.TaskID) bool {
	set := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		set[t.ID] = true
	}
	return func(id mtt.TaskID) bool { return set[id] }
}

func noteExistsFn(notes []mtt.Note) func(mtt.NoteSlug) bool {
	set := make(map[mtt.NoteSlug]bool, len(notes))
	for _, n := range notes {
		set[n.Slug] = true
	}
	return func(s mtt.NoteSlug) bool { return set[s] }
}

// verifyOne resolves one ref against the two stores (the YAML KB is always wired) —
// for the single-op warn on write.
func verifyOne(root string, r mtt.Ref) core.RefStatus {
	tasks, _ := yaml.NewTaskStore(root).List()
	notes, _ := yaml.NewKnowledgeStore(root).ListNotes()
	return core.VerifyRef(r, taskExistsFn(tasks), noteExistsFn(notes))
}

// warnIfNotOK warns (stderr, warn-not-block) about a DANGLING ref, or a note ref
// that could not be verified (no KB wired). A well-formed url is expected to be
// unverified and does NOT warn (DESIGN: "warn about a *dangling* reference").
func warnIfNotOK(cmd *cobra.Command, r mtt.Ref, st core.RefStatus) {
	if st == core.RefDangling || (st == core.RefUnverified && r.Kind == mtt.RefNote) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s:%s is %s\n", r.Kind, r.ID, st)
	}
}

// formatRefsBacklinks renders the human refs:/backlinks: block for show / note show,
// or "" when the carrier has neither (so a ref-less show is byte-unchanged). Lines
// are indented under a 2-space header to match formatTask's field style.
func formatRefsBacklinks(refs []mtt.Ref, back []core.Referent, te func(mtt.TaskID) bool, ne func(mtt.NoteSlug) bool) string {
	if len(refs) == 0 && len(back) == 0 {
		return ""
	}
	var b strings.Builder
	if len(refs) > 0 {
		b.WriteString("  refs:\n")
		for _, r := range refs {
			fmt.Fprintf(&b, "    %s\n", refLine(r, core.VerifyRef(r, te, ne)))
		}
	}
	if len(back) > 0 {
		b.WriteString("  backlinks:\n")
		for _, r := range back {
			if r.Label != "" {
				fmt.Fprintf(&b, "    %s:%s  (%s)\n", r.Carrier, r.ID, r.Label)
			} else {
				fmt.Fprintf(&b, "    %s:%s\n", r.Carrier, r.ID)
			}
		}
	}
	return b.String()
}

// writeRefsAndBacklinks renders a carrier's outgoing refs (verified) + incoming
// backlinks for `ref list` / `note ref list`, in text (always both headers) or
// (--json) {refs:[...], backlinks:[...]} (both non-null).
func writeRefsAndBacklinks(cmd *cobra.Command, carrierKind mtt.RefKind, carrierID string, refs []mtt.Ref, tasks []mtt.Task, notes []mtt.Note) error {
	te, ne := taskExistsFn(tasks), noteExistsFn(notes)
	back := core.NewBacklinks(tasks, notes).To(carrierKind, carrierID)
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), struct {
			Refs      []refJSON      `json:"refs"`
			Backlinks []backlinkJSON `json:"backlinks"`
		}{verifiedRefsJSON(refs, te, ne), toBacklinkJSON(back)})
	}
	var b strings.Builder
	b.WriteString("refs:\n")
	if len(refs) == 0 {
		b.WriteString("  (none)\n") // match dep list — no bare empty header (c14)
	}
	for _, r := range refs {
		fmt.Fprintf(&b, "  %s\n", refLine(r, core.VerifyRef(r, te, ne)))
	}
	b.WriteString("backlinks:\n")
	if len(back) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, r := range back {
		if r.Label != "" {
			fmt.Fprintf(&b, "  %s:%s  (%s)\n", r.Carrier, r.ID, r.Label)
		} else {
			fmt.Fprintf(&b, "  %s:%s\n", r.Carrier, r.ID)
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return err
}

// newRefCmd builds `mtt ref` with add/rm/list (the dep pattern) for TASK carriers.
func newRefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ref", Short: "Manage references on tasks (note/task/url)"}
	cmd.AddCommand(newRefAddCmd(), newRefRmCmd(), newRefListCmd())
	return cmd
}

func newRefAddCmd() *cobra.Command {
	var label string
	var noRun bool
	cmd := &cobra.Command{
		Use:   "add <id> <kind>:<target>",
		Short: "Add a reference to a task",
		Args:  twoIDs("provide a task id and <kind>:<target> (example: mtt ref add t2 task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			ref.Label = label
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			opts, err := eventOptions(cmd, noRun, settings.Author)
			if err != nil {
				return err
			}
			ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
			if err != nil {
				return err
			}
			defer closeOut()
			task, err := core.NewRefEditor(yaml.NewTaskStore(root), time.Now, ev).AddRef(id, ref, cmd.Flags().Changed("label"), opts)
			if err != nil && !errors.As(err, new(*core.PostActionError)) {
				if isNotFound(err) {
					return taskNotFound(id)
				}
				return err
			}
			evErr := err
			st := verifyOne(root, ref)
			warnIfNotOK(cmd, ref, st)
			return finishMutation(cmd, evErr, func() error {
				if jsonFlag(cmd) {
					return writeJSON(cmd.OutOrStdout(), toRefJSON(ref, st))
				}
				_, werr := fmt.Fprintf(cmd.OutOrStdout(), "added %s:%s to %s\n", ref.Kind, ref.ID, task.ID)
				return werr
			})
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "annotate the reference")
	addNoRunFlag(cmd, &noRun)
	return cmd
}

func newRefRmCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "rm <id> <kind>:<target>",
		Short: "Remove a reference from a task (idempotent)",
		Args:  twoIDs("provide a task id and <kind>:<target> (example: mtt ref rm t2 task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			opts, err := eventOptions(cmd, noRun, settings.Author)
			if err != nil {
				return err
			}
			ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
			if err != nil {
				return err
			}
			defer closeOut()
			rmErr := func() error {
				_, e := core.NewRefEditor(yaml.NewTaskStore(root), time.Now, ev).RemoveRef(id, ref.Kind, ref.ID, opts)
				return e
			}()
			if rmErr != nil && !errors.As(rmErr, new(*core.PostActionError)) {
				if isNotFound(rmErr) {
					return taskNotFound(id)
				}
				return rmErr
			}
			return finishMutation(cmd, rmErr, func() error {
				_, werr := fmt.Fprintf(cmd.OutOrStdout(), "removed %s:%s from %s\n", ref.Kind, ref.ID, id)
				return werr
			})
		},
	}
	addNoRunFlag(cmd, &noRun)
	return cmd
}

func newRefListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <id>",
		Short: "List a task's references and backlinks",
		Args:  oneID("provide a task id (example: mtt ref list t2)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			store := yaml.NewTaskStore(root)
			task, err := store.Get(id)
			if err != nil {
				if isNotFound(err) {
					return taskNotFound(id)
				}
				return err
			}
			tasks, err := store.List()
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			return writeRefsAndBacklinks(cmd, mtt.RefTask, string(id), task.Refs, tasks, notes)
		},
	}
}
