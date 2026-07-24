package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// parseRefFlags parses repeated --ref <kind>:<target> values (creation-time).
func parseRefFlags(vals []string) ([]mtt.Ref, error) {
	out := make([]mtt.Ref, 0, len(vals))
	for _, v := range vals {
		r, err := parseRefArg(v)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// newNoteRefCmd builds `mtt note ref` with add/rm/list for NOTE carriers (the ref
// group's note analogue).
func newNoteRefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ref", Short: "Manage references on a note (note/task/url)"}
	cmd.AddCommand(newNoteRefAddCmd(), newNoteRefRmCmd(), newNoteRefListCmd())
	return cmd
}

func newNoteRefAddCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "add <slug> <kind>:<target>",
		Short: "Add a reference to a note",
		Args:  twoIDs("provide a note slug and <kind>:<target> (example: mtt note ref add a task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			ref.Label = label
			note, err := core.NewNoteRefEditor(yaml.NewKnowledgeStore(root), time.Now, nil).AddRef(slug, ref, cmd.Flags().Changed("label"), core.EventOptions{})
			if err != nil {
				if isNotFound(err) {
					return noteNotFound(slug)
				}
				return err
			}
			st := verifyOne(root, ref)
			warnIfNotOK(cmd, ref, st)
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toRefJSON(ref, st))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added %s:%s to %s\n", ref.Kind, ref.ID, note.Slug)
			return err
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "annotate the reference")
	return cmd
}

func newNoteRefRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <slug> <kind>:<target>",
		Short: "Remove a reference from a note (idempotent)",
		Args:  twoIDs("provide a note slug and <kind>:<target> (example: mtt note ref rm a task:t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			ref, err := parseRefArg(args[1])
			if err != nil {
				return err
			}
			if _, err := core.NewNoteRefEditor(yaml.NewKnowledgeStore(root), time.Now, nil).RemoveRef(slug, ref.Kind, ref.ID, core.EventOptions{}); err != nil {
				if isNotFound(err) {
					return noteNotFound(slug)
				}
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s:%s from %s\n", ref.Kind, ref.ID, slug)
			return err
		},
	}
}

func newNoteRefListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <slug>",
		Short: "List a note's references and backlinks",
		Args:  oneID("provide a note slug (example: mtt note ref list auth-design)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			slug, err := mtt.NewNoteSlug(args[0])
			if err != nil {
				return err
			}
			kb := yaml.NewKnowledgeStore(root)
			note, err := kb.GetNote(slug)
			if err != nil {
				if isNotFound(err) {
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
			return writeRefsAndBacklinks(cmd, mtt.RefNote, string(slug), note.Refs, tasks, notes)
		},
	}
}
