package commit

import "context"

// Backend executes a commit plan.
type Backend interface {
	Execute(ctx context.Context, plan *Plan, conventional bool) (*Result, error)
}

// Result holds the outcome of a commit execution.
type Result struct {
	SHA     string
	Message string
	Files   []string // actual staged files (from git diff --cached --name-only)
	Pushed  bool
	NoOp    bool
}
