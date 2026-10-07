package git

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Change describes a verified before/after transition; it contains no diff or
// repository file contents.
type Change struct {
	Kind     string
	Summary  string
	Resource string
}

func Compare(command string, before, after Context) []Change {
	if !before.IsRepository || !after.IsRepository || before.Root != after.Root {
		return nil
	}
	repo := filepath.Base(after.Root)
	var changes []Change
	branchChanged := before.Branch != after.Branch && before.Branch != "" && after.Branch != ""
	if branchChanged {
		changes = append(changes, Change{"branch_switch", fmt.Sprintf("Branch: %s → %s", before.Branch, after.Branch), repo})
	}
	if before.Commit != after.Commit && after.Commit != "" && !branchChanged {
		kind := "head_changed"
		if gitSubcommand(command) == "commit" {
			kind = "commit"
		}
		changes = append(changes, Change{kind, fmt.Sprintf("HEAD: %s → %s", short(before.Commit), short(after.Commit)), repo})
	}
	// Worktree/index transitions are useful even when both sides are dirty.
	// Do not report a second card when the commit or branch transition already
	// explains the command's primary effect.
	if len(changes) == 0 && !sameFiles(before.Files, after.Files) {
		paths := changedFiles(before.Files, after.Files)
		if len(paths) > 0 {
			summary := fmt.Sprintf("Files: %d status changes", len(paths))
			if len(paths) <= 4 {
				summary += " (" + strings.Join(paths, ", ") + ")"
			}
			changes = append(changes, Change{"files_changed", summary, repo})
		}
	}
	if before.Dirty != after.Dirty {
		state := "clean"
		if after.Dirty {
			state = "dirty"
		}
		// A commit already represents its own clean-tree effect; avoid a duplicate
		// Git card for the most common commit operation.
		if len(changes) == 0 {
			changes = append(changes, Change{"working_tree", fmt.Sprintf("Working tree: %s", state), repo})
		}
	}
	return changes
}
func short(oid string) string {
	if oid == "" {
		return "(unborn)"
	}
	if len(oid) > 10 {
		return oid[:10]
	}
	return oid
}
func gitSubcommand(command string) string {
	fields := strings.Fields(command)
	for len(fields) > 0 && (fields[0] == "sudo" || fields[0] == "command") {
		fields = fields[1:]
	}
	if len(fields) < 2 || filepath.Base(fields[0]) != "git" {
		return ""
	}
	return fields[1]
}

// MutatingCommand conservatively selects commands that warrant a pre-state.
func MutatingCommand(command string) bool {
	switch gitSubcommand(command) {
	case "add", "commit", "switch", "checkout", "merge", "rebase", "reset", "restore", "rm", "mv", "stash", "cherry-pick", "revert", "pull":
		return true
	default:
		return false
	}
}

func sameFiles(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a := append([]string(nil), left...)
	b := append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func changedFiles(before, after []string) []string {
	old := make(map[string]bool, len(before))
	for _, f := range before {
		old[f] = true
	}
	var changed []string
	for _, f := range after {
		if !old[f] {
			changed = append(changed, f)
		}
	}
	if len(changed) == 0 && len(before) > 0 && len(after) == 0 {
		return []string{"(working tree clean)"}
	}
	sort.Strings(changed)
	return changed
}
