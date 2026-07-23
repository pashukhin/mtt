package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newNoteCmd builds `mtt note` with add/list/show/edit/rm subcommands (the dep pattern).
func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Manage knowledge-base notes",
	}
	cmd.AddCommand(newNoteAddCmd(), newNoteListCmd(), newNoteShowCmd(), newNoteEditCmd(), newNoteRmCmd(), newNoteRefCmd())
	return cmd
}

// noteJSON is the CLI's machine-readable view of a note: slug always present
// (identity), tags a non-null array ([] when empty).
type noteJSON struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title,omitempty"`
	Tags     []string `json:"tags"`
	Priority string   `json:"priority,omitempty"`
	Body     string   `json:"body"`
	Created  string   `json:"created"`
	Updated  string   `json:"updated"`
}

func toNoteJSON(n mtt.Note) noteJSON {
	tags := n.Tags
	if tags == nil {
		tags = []string{}
	}
	return noteJSON{
		Slug:     string(n.Slug),
		Title:    n.Title,
		Tags:     tags,
		Priority: string(n.Priority),
		Body:     n.Body,
		Created:  n.Created.UTC().Format(time.RFC3339),
		Updated:  n.Updated.UTC().Format(time.RFC3339),
	}
}

// readNoteBody resolves the body from the mutually-exclusive --body / --file
// (--file - = stdin). None provided -> "" (empty body allowed).
func readNoteBody(cmd *cobra.Command, body, file string) (string, error) {
	bodySet, fileSet := cmd.Flags().Changed("body"), cmd.Flags().Changed("file")
	if bodySet && fileSet {
		return "", errors.New("provide at most one of --body or --file")
	}
	switch {
	case bodySet:
		return body, nil
	case fileSet:
		if file == "-" {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return "", fmt.Errorf("read stdin: %w", err)
			}
			return string(data), nil
		}
		// G304: file is the user's own --file argument to a local CLI; reading the
		// path they named is the intended behavior (they already have the process's
		// filesystem access), not a disclosure vector.
		data, err := os.ReadFile(file) //nolint:gosec
		if err != nil {
			return "", fmt.Errorf("read %s: %w", file, err)
		}
		return string(data), nil
	}
	return "", nil
}

func newNoteAddCmd() *cobra.Command {
	var (
		title, body, file, priority string
		tags                        []string
		refVals                     []string
	)
	cmd := &cobra.Command{
		Use:   "add <slug>",
		Short: "Create a knowledge note",
		Args:  oneID("provide exactly one slug (example: mtt note add auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			normTags, err := toTags(tags)
			if err != nil {
				return err
			}
			prio, err := parsePriority(priority)
			if err != nil {
				return err
			}
			refs, err := parseRefFlags(refVals)
			if err != nil {
				return err
			}
			b, err := readNoteBody(cmd, body, file)
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			note, err := core.NewNoteAdder(yaml.NewKnowledgeStore(root), time.Now).Add(core.NoteParams{Slug: slug, Title: title, Tags: normTags, Priority: prio, Body: b, Refs: refs})
			if err != nil {
				return err
			}
			for _, r := range refs {
				warnIfNotOK(cmd, r, verifyOne(root, r))
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", note.Slug)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "note title")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "add a tag (repeatable, comma-separated)")
	cmd.Flags().StringVar(&priority, "priority", "", "note priority: high|medium|low")
	cmd.Flags().StringVar(&body, "body", "", "note body (markdown)")
	cmd.Flags().StringVar(&file, "file", "", "read the body from a file ('-' for stdin)")
	cmd.Flags().StringArrayVar(&refVals, "ref", nil, "add a reference <kind>:<target> (repeatable)")
	return cmd
}

func newNoteListCmd() *cobra.Command {
	var tags, priorities []string
	var sortKey string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List knowledge notes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			normTags, err := toTags(tags)
			if err != nil {
				return err
			}
			prios, err := toPriorities(priorities)
			if err != nil {
				return err
			}
			switch sortKey {
			case "", string(core.SortCreated), string(core.SortUpdated), string(core.SortPriority):
			default:
				return fmt.Errorf("invalid --sort %q: want created|updated|priority", sortKey)
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			notes, err := yaml.NewKnowledgeStore(root).ListNotes()
			if err != nil {
				return err
			}
			sel := core.SelectNotes(notes, core.NoteFilter{Tags: normTags, Priorities: prios, Sort: core.SortKey(sortKey)})
			if jsonFlag(cmd) {
				out := make([]noteJSON, 0, len(sel))
				for _, n := range sel {
					out = append(out, toNoteJSON(n))
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}
			var b strings.Builder
			for _, n := range sel {
				fmt.Fprintf(&b, "%s\n", noteLine(n))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable, comma-separated; OR within)")
	cmd.Flags().StringArrayVar(&priorities, "priority", nil, "filter by priority: high|medium|low (repeatable)")
	cmd.Flags().StringVar(&sortKey, "sort", "", "sort order: created|updated|priority (default created)")
	return cmd
}

// noteLine is the one-row list formatter: slug, title (or (untitled)), optional tags.
func noteLine(n mtt.Note) string {
	title := n.Title
	if title == "" {
		title = "(untitled)"
	}
	if len(n.Tags) > 0 {
		return fmt.Sprintf("%s  %s  [%s]", n.Slug, title, strings.Join(n.Tags, ", "))
	}
	return fmt.Sprintf("%s  %s", n.Slug, title)
}

// noteShowJSON is `mtt note show --json`: the lean note view plus its verified refs
// and computed backlinks (the lean noteJSON stays for note list/add/edit).
type noteShowJSON struct {
	noteJSON
	Refs      []refJSON      `json:"refs,omitempty"`
	Backlinks []backlinkJSON `json:"backlinks,omitempty"`
}

func newNoteShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show a knowledge note (frontmatter + body)",
		Args:  oneID("provide exactly one slug (example: mtt note show auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			kb := yaml.NewKnowledgeStore(root)
			note, err := kb.GetNote(slug)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return noteNotFound(slug)
				}
				return err
			}
			notes, err := kb.ListNotes()
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			te, ne := taskExistsFn(tasks), noteExistsFn(notes)
			back := core.NewBacklinks(tasks, notes).To(mtt.RefNote, string(slug))
			if jsonFlag(cmd) {
				sj := noteShowJSON{noteJSON: toNoteJSON(note)}
				if len(note.Refs) > 0 {
					sj.Refs = verifiedRefsJSON(note.Refs, te, ne)
				}
				if len(back) > 0 {
					sj.Backlinks = toBacklinkJSON(back)
				}
				return writeJSON(cmd.OutOrStdout(), sj)
			}
			if err := writeNote(cmd, note); err != nil {
				return err
			}
			if block := formatRefsBacklinks(note.Refs, back, te, ne); block != "" {
				_, err = fmt.Fprint(cmd.OutOrStdout(), block)
			}
			return err
		},
	}
}

// writeNote renders a note for humans: a header then the body.
func writeNote(cmd *cobra.Command, n mtt.Note) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", n.Slug)
	if n.Title != "" {
		fmt.Fprintf(&b, "  title:   %s\n", n.Title)
	}
	if len(n.Tags) > 0 {
		fmt.Fprintf(&b, "  tags:    %s\n", strings.Join(n.Tags, ", "))
	}
	if n.Priority != "" {
		fmt.Fprintf(&b, "  priority: %s\n", n.Priority)
	}
	fmt.Fprintf(&b, "  created: %s\n", n.Created.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  updated: %s\n", n.Updated.UTC().Format(time.RFC3339))
	if n.Body != "" {
		fmt.Fprintf(&b, "\n%s", n.Body)
		if !strings.HasSuffix(n.Body, "\n") {
			b.WriteString("\n")
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return err
}

func newNoteEditCmd() *cobra.Command {
	var (
		title, body, file, priority string
		tags                        []string
	)
	cmd := &cobra.Command{
		Use:   "edit <slug>",
		Short: "Edit a note's title, tags, priority, and/or body",
		Args:  oneID("provide exactly one slug (example: mtt note edit auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			var p core.NoteEditParams
			if cmd.Flags().Changed("title") {
				p.Title = &title
			}
			if cmd.Flags().Changed("tag") {
				normTags, err := toTags(tags)
				if err != nil {
					return err
				}
				p.Tags = &normTags
			}
			if cmd.Flags().Changed("priority") {
				pr, err := parsePriority(priority)
				if err != nil {
					return err
				}
				p.Priority = &pr
			}
			if cmd.Flags().Changed("body") || cmd.Flags().Changed("file") {
				b, err := readNoteBody(cmd, body, file)
				if err != nil {
					return err
				}
				p.Body = &b
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			note, err := core.NewNoteEditor(yaml.NewKnowledgeStore(root), time.Now).Edit(slug, p)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return noteNotFound(slug)
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", note.Slug)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "replace the tag set (repeatable, comma-separated; --tag '' clears it)")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority: high|medium|low (empty string clears it)")
	cmd.Flags().StringVar(&body, "body", "", "new body (markdown)")
	cmd.Flags().StringVar(&file, "file", "", "read the new body from a file ('-' for stdin)")
	return cmd
}

func newNoteRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a knowledge note (refuses if referenced; --force overrides)",
		Args:  oneID("provide exactly one slug (example: mtt note rm auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			kb := yaml.NewKnowledgeStore(root)
			var note mtt.Note
			if jsonFlag(cmd) { // capture before delete so --json can echo the removed note
				note, err = kb.GetNote(slug)
				if err != nil {
					if errors.Is(err, mtt.ErrNotFound) {
						return noteNotFound(slug)
					}
					return err
				}
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			notes, err := kb.ListNotes()
			if err != nil {
				return err
			}
			referents := referentIDs(core.NewBacklinks(tasks, notes).To(mtt.RefNote, string(slug)), slug)
			_, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			_, by, why, err := resolveAttribution(cmd, settings.Author)
			if err != nil {
				return err
			}
			if err := core.NewNoteRemover(kb, yaml.NewAuditStore(root), time.Now).Remove(slug, referents, force, by, why); err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return noteNotFound(slug)
				}
				return err // ErrMissingAttribution -> exit 2; referenced-refusal -> exit 1
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toNoteJSON(note))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", slug)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if referenced (leaves dangling refs)")
	return cmd
}

// referentIDs formats backlink referents as strings (note carriers labelled),
// EXCLUDING the note's own self-reference (a note referencing itself must not block
// its own delete — symmetric with the task guard's subgraph-ignore of the deletion
// set).
func referentIDs(refs []core.Referent, self mtt.NoteSlug) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Carrier == mtt.RefNote {
			if r.ID == string(self) {
				continue // self-ref never blocks
			}
			out = append(out, "note:"+r.ID)
		} else {
			out = append(out, r.ID)
		}
	}
	return out
}
