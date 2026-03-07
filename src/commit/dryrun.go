package commit

import (
	"context"
	"os/exec"
	"strings"
)

// DryRunBackend prints the commit plan without side effects.
type DryRunBackend struct {
	RootDir string
}

// Execute simulates the commit and returns what would happen.
func (d *DryRunBackend) Execute(_ context.Context, plan *Plan, conventional bool) (*Result, error) {
	var files []string
	var err error

	switch plan.StageMode {
	case StageExplicit:
		files = plan.Paths
	case StageAll:
		files, err = gitStatusFiles(d.RootDir)
		if err != nil {
			return nil, err
		}
	case StageStaged:
		files, err = gitStagedFiles(d.RootDir)
		if err != nil {
			return nil, err
		}
	}

	return &Result{
		Message: plan.Message(conventional),
		Files:   files,
		NoOp:    len(files) == 0,
	}, nil
}

// gitStatusFiles returns all changed files (staged + unstaged + untracked).
func gitStatusFiles(rootDir string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) >= 4 {
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}

// gitStagedFiles returns files currently in the staging area.
func gitStagedFiles(rootDir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
