package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Context is intentionally metadata-only: no patches, commit messages or file contents.
type Context struct {
	IsRepository bool     `json:"is_repository"`
	Root         string   `json:"root"`
	Branch       string   `json:"branch"`
	Commit       string   `json:"commit"`
	Dirty        bool     `json:"dirty"`
	Files        []string `json:"files,omitempty"`
}

// Detect uses porcelain-v2 to obtain branch, HEAD and working-tree status in
// one invocation. A second invocation locates the repository root, including
// when Detect is called from a subdirectory or linked worktree.
func Detect(workingDirectory string) (Context, error) {
	output, err := runGit(workingDirectory, "--no-optional-locks", "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
	if err != nil {
		if isNotRepository(err) && strings.Contains(err.Error(), "not a git repository") {
			return Context{}, nil
		}
		return Context{}, err
	}
	result := ParseStatus(output)
	root, err := runGit(workingDirectory, "rev-parse", "--show-toplevel")
	if err != nil {
		return Context{}, err
	}
	result.Root = root
	result.IsRepository = true
	return result, nil
}

// ParseStatus parses porcelain-v2's stable branch headers and change records.
// Paths are only retained for concise metadata; rename sources and diff
// contents are deliberately omitted.
func ParseStatus(output string) Context {
	var result Context
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			result.Branch = strings.TrimPrefix(line, "# branch.head ")
			if result.Branch == "(detached)" {
				result.Branch = "HEAD (detached)"
			}
		case strings.HasPrefix(line, "# branch.oid "):
			result.Commit = strings.TrimPrefix(line, "# branch.oid ")
			if result.Commit == "(initial)" {
				result.Commit = ""
			}
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "), strings.HasPrefix(line, "u "):
			result.Dirty = true
			// Field counts follow git-status porcelain-v2: the path is the last field.
			fields := strings.Fields(line)
			if len(fields) > 0 {
				if len(fields) > 8 {
					pathStart := 8
					if fields[0] == "2" {
						pathStart = 9
					}
					if pathStart < len(fields) {
						result.Files = append(result.Files, fields[1]+" "+strings.Join(fields[pathStart:], " "))
					}
				}
			}
		case strings.HasPrefix(line, "? "):
			result.Dirty = true
			result.Files = append(result.Files, "?? "+strings.TrimPrefix(line, "? "))
		}
	}
	return result
}

func runGit(workingDirectory string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	commandArgs := append([]string{"-C", workingDirectory}, args...)
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func isNotRepository(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 128
}
