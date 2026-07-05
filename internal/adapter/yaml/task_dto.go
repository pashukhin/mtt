package yaml

import (
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// timeLayout is the on-disk timestamp format: RFC3339 UTC, second precision.
const timeLayout = time.RFC3339

// ymlTask is the on-disk DTO for a task: yaml tags + omitempty for optional and
// reserved fields; field order matches mtt.Task for a deterministic diff.
type ymlTask struct {
	ID          string            `yaml:"id"`
	Type        string            `yaml:"type"`
	Title       string            `yaml:"title,omitempty"`
	Status      string            `yaml:"status"`
	Parent      string            `yaml:"parent,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Refs        []ymlRef          `yaml:"refs,omitempty"`
	Created     string            `yaml:"created"`
	Updated     string            `yaml:"updated"`
	Description string            `yaml:"description,omitempty"`
	Comments    []ymlComment      `yaml:"comments,omitempty"`
	History     []ymlHistoryEntry `yaml:"history,omitempty"`
}

type ymlRef struct {
	Kind  string `yaml:"kind"`
	ID    string `yaml:"id"`
	Label string `yaml:"label,omitempty"`
}

type ymlComment struct {
	ID      int          `yaml:"id"`
	Author  string       `yaml:"author,omitempty"`
	Created string       `yaml:"created"`
	Body    string       `yaml:"body,omitempty"`
	Refs    []ymlRef     `yaml:"refs,omitempty"`
	Replies []ymlComment `yaml:"replies,omitempty"`
}

type ymlHistoryEntry struct {
	At     string     `yaml:"at"`
	By     string     `yaml:"by,omitempty"`
	Role   string     `yaml:"role,omitempty"`
	From   string     `yaml:"from"`
	To     string     `yaml:"to"`
	Checks []ymlCheck `yaml:"checks,omitempty"`
}

type ymlCheck struct {
	Cmd  string `yaml:"cmd"`
	Exit int    `yaml:"exit"`
}

func fmtTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func fromDomainRefs(rs []mtt.Ref) []ymlRef {
	if len(rs) == 0 {
		return nil
	}
	out := make([]ymlRef, len(rs))
	for i, r := range rs {
		out[i] = ymlRef{Kind: string(r.Kind), ID: r.ID, Label: r.Label}
	}
	return out
}

func fromDomainComments(cs []mtt.Comment) []ymlComment {
	if len(cs) == 0 {
		return nil
	}
	out := make([]ymlComment, len(cs))
	for i, c := range cs {
		out[i] = ymlComment{ID: c.ID, Author: c.Author, Created: fmtTime(c.Created), Body: c.Body,
			Refs: fromDomainRefs(c.Refs), Replies: fromDomainComments(c.Replies)}
	}
	return out
}

func fromDomainHistory(hs []mtt.HistoryEntry) []ymlHistoryEntry {
	if len(hs) == 0 {
		return nil
	}
	out := make([]ymlHistoryEntry, len(hs))
	for i, h := range hs {
		var checks []ymlCheck
		if len(h.Checks) > 0 {
			checks = make([]ymlCheck, len(h.Checks))
			for j, ch := range h.Checks {
				checks[j] = ymlCheck{Cmd: ch.Cmd, Exit: ch.Exit}
			}
		}
		out[i] = ymlHistoryEntry{At: fmtTime(h.At), By: h.By, Role: h.Role, From: h.From, To: h.To, Checks: checks}
	}
	return out
}

// fromDomainTask maps the pure domain task to its on-disk DTO.
func fromDomainTask(t mtt.Task) ymlTask {
	return ymlTask{
		ID: string(t.ID), Type: string(t.Type), Title: t.Title, Status: t.Status, Parent: string(t.Parent),
		Tags: t.Tags, DependsOn: fromDomainDeps(t.DependsOn), Refs: fromDomainRefs(t.Refs),
		Created: fmtTime(t.Created), Updated: fmtTime(t.Updated), Description: t.Description,
		Comments: fromDomainComments(t.Comments), History: fromDomainHistory(t.History),
	}
}

// fromDomainDeps maps typed dependency ids to their on-disk string form.
func fromDomainDeps(ids []mtt.TaskID) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// toDomainDeps maps on-disk dependency strings to typed ids (optional field, so
// no non-empty guard — plain conversion).
func toDomainDeps(ids []string) []mtt.TaskID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]mtt.TaskID, len(ids))
	for i, id := range ids {
		out[i] = mtt.TaskID(id)
	}
	return out
}

func toDomainRefs(rs []ymlRef) []mtt.Ref {
	if len(rs) == 0 {
		return nil
	}
	out := make([]mtt.Ref, len(rs))
	for i, r := range rs {
		out[i] = mtt.Ref{Kind: mtt.RefKind(r.Kind), ID: r.ID, Label: r.Label}
	}
	return out
}

func toDomainComments(cs []ymlComment) ([]mtt.Comment, error) {
	if len(cs) == 0 {
		return nil, nil
	}
	out := make([]mtt.Comment, len(cs))
	for i, c := range cs {
		created, err := parseTime(c.Created)
		if err != nil {
			return nil, err
		}
		replies, err := toDomainComments(c.Replies)
		if err != nil {
			return nil, err
		}
		out[i] = mtt.Comment{ID: c.ID, Author: c.Author, Created: created, Body: c.Body,
			Refs: toDomainRefs(c.Refs), Replies: replies}
	}
	return out, nil
}

func toDomainHistory(hs []ymlHistoryEntry) ([]mtt.HistoryEntry, error) {
	if len(hs) == 0 {
		return nil, nil
	}
	out := make([]mtt.HistoryEntry, len(hs))
	for i, h := range hs {
		at, err := parseTime(h.At)
		if err != nil {
			return nil, err
		}
		var checks []mtt.Check
		if len(h.Checks) > 0 {
			checks = make([]mtt.Check, len(h.Checks))
			for j, ch := range h.Checks {
				checks[j] = mtt.Check{Cmd: ch.Cmd, Exit: ch.Exit}
			}
		}
		out[i] = mtt.HistoryEntry{At: at, By: h.By, Role: h.Role, From: h.From, To: h.To, Checks: checks}
	}
	return out, nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t.UTC(), nil
}

// toDomain maps the on-disk DTO back to the pure domain task.
func (yt ymlTask) toDomain() (mtt.Task, error) {
	id, err := mtt.NewTaskID(yt.ID)
	if err != nil {
		return mtt.Task{}, err
	}
	typ, err := mtt.NewTypeName(yt.Type)
	if err != nil {
		return mtt.Task{}, err
	}
	created, err := parseTime(yt.Created)
	if err != nil {
		return mtt.Task{}, err
	}
	updated, err := parseTime(yt.Updated)
	if err != nil {
		return mtt.Task{}, err
	}
	comments, err := toDomainComments(yt.Comments)
	if err != nil {
		return mtt.Task{}, err
	}
	history, err := toDomainHistory(yt.History)
	if err != nil {
		return mtt.Task{}, err
	}
	return mtt.Task{
		ID: id, Type: typ, Title: yt.Title, Status: yt.Status, Parent: mtt.TaskID(yt.Parent),
		Tags: yt.Tags, DependsOn: toDomainDeps(yt.DependsOn), Refs: toDomainRefs(yt.Refs),
		Created: created, Updated: updated, Description: yt.Description,
		Comments: comments, History: history,
	}, nil
}
