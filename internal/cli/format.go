package cli

import (
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// taskLine renders a task as one compact row: "<id>  <type>  [<status>]  <title>"
// (the title is omitted when empty). Shared by `list` and `tree` so both agree.
func taskLine(t mtt.Task) string {
	s := fmt.Sprintf("%s  %s  [%s]", t.ID, t.Type, t.Status)
	if t.Title != "" {
		s += "  " + t.Title
	}
	return s
}
