package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sofmeright/stagefreight/src/commit"
	"github.com/sofmeright/stagefreight/src/output"
)

var (
	commitType    string
	commitScope   string
	commitMessage string
	commitBody    string
	commitAdd     []string
	commitAll     bool
	commitBreak   bool
	commitSkipCI  bool
	commitPush    bool
	commitRemote  string
	commitRefspec string
	commitDryRun  bool
	commitSignOff bool
)

var commitCmd = &cobra.Command{
	Use:   "commit [summary]",
	Short: "Create a conventional commit from staged or specified files",
	Long: `Create a git commit with conventional commit formatting.

Summary can be provided as a positional argument or via --message.
Files are staged via --add flags, --all, or from the existing staging area.

In CI environments, the push refspec is auto-detected from CI_COMMIT_REF_NAME
or CI_COMMIT_BRANCH. Use --refspec for explicit control.

Examples:
  stagefreight commit -t docs -m "refresh generated docs"
  stagefreight commit -t docs "refresh generated docs"
  stagefreight commit -t fix -m "handle edge case"
  stagefreight commit --dry-run -t docs -m "test" --add docs/
  stagefreight commit -t docs -m "refresh docs" --push --refspec HEAD:refs/heads/main`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommit,
}

func init() {
	commitCmd.Flags().StringVarP(&commitType, "type", "t", "", "commit type (e.g. feat, fix, docs, chore)")
	commitCmd.Flags().StringVarP(&commitScope, "scope", "s", "", "commit scope")
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "commit summary message")
	commitCmd.Flags().StringVar(&commitBody, "body", "", "commit body (appended after blank line)")
	commitCmd.Flags().StringSliceVar(&commitAdd, "add", nil, "files/dirs to stage (repeatable, supports globs)")
	commitCmd.Flags().BoolVar(&commitAll, "all", false, "stage all changes (git add -A)")
	commitCmd.Flags().BoolVar(&commitBreak, "breaking", false, "mark as breaking change (!)")
	commitCmd.Flags().BoolVar(&commitSkipCI, "skip-ci", false, "append [skip ci] to subject line")
	commitCmd.Flags().BoolVar(&commitPush, "push", false, "push after commit")
	commitCmd.Flags().StringVar(&commitRemote, "remote", "origin", "git remote for push")
	commitCmd.Flags().StringVar(&commitRefspec, "refspec", "", "push refspec (e.g. HEAD:refs/heads/main)")
	commitCmd.Flags().BoolVar(&commitDryRun, "dry-run", false, "show what would be committed without executing")
	commitCmd.Flags().BoolVar(&commitSignOff, "sign-off", false, "add Signed-off-by trailer")

	rootCmd.AddCommand(commitCmd)
}

func runCommit(cmd *cobra.Command, args []string) error {
	start := time.Now()
	ctx := context.Background()

	// Resolve summary: positional arg OR --message flag
	summary := commitMessage
	if len(args) > 0 {
		if summary != "" {
			return fmt.Errorf("cannot use both positional summary and --message flag")
		}
		summary = args[0]
	}

	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	registry := commit.NewTypeRegistry(cfg.Commit.Types)

	// Build planner options
	opts := commit.PlannerOptions{
		Type:    commitType,
		Scope:   commitScope,
		Message: summary,
		Body:    commitBody,
		Paths:   commitAdd,
		All:     commitAll,
		SignOff: commitSignOff,
		Remote:  commitRemote,
		Refspec: commitRefspec,
	}
	if cmd.Flags().Changed("breaking") {
		opts.Breaking = commitBreak
	}
	if cmd.Flags().Changed("skip-ci") {
		opts.SkipCI = &commitSkipCI
	}
	if cmd.Flags().Changed("push") {
		opts.Push = &commitPush
	}

	plan, err := commit.BuildPlan(opts, cfg.Commit, registry, rootDir)
	if err != nil {
		return err
	}

	// Select backend
	var backend commit.Backend
	if commitDryRun {
		backend = &commit.DryRunBackend{RootDir: rootDir}
	} else {
		backend = &commit.GitBackend{RootDir: rootDir}
	}

	result, err := backend.Execute(ctx, plan, cfg.Commit.Conventional)
	if err != nil {
		return err
	}

	// Render output
	elapsed := time.Since(start)
	useColor := output.UseColor()
	w := os.Stdout
	sec := output.NewSection(w, "Commit", elapsed, useColor)

	// Type display
	typeDisplay := plan.Type
	if plan.Scope != "" {
		typeDisplay += fmt.Sprintf("(%s)", plan.Scope)
	}
	if plan.Breaking {
		typeDisplay += "!"
	}
	sec.Row("%-16s%s", "type", typeDisplay)

	if result.NoOp {
		sec.Row("%-16s%s", "status", "nothing to commit")
		sec.Close()
		return nil
	}

	// Message
	sec.Row("%-16s%s", "message", plan.Subject(cfg.Commit.Conventional))

	if commitDryRun {
		sec.Row("%-16s%s", "mode", "dry-run")
	}

	// Staged files
	sec.Row("%-16s%d files", "staged", len(result.Files))
	for _, f := range result.Files {
		output.RowStatus(sec, f, "", "success", useColor)
	}

	// SHA
	if result.SHA != "" {
		sec.Row("%-16s%s", "sha", result.SHA)
	}

	// Push status
	if plan.Push.Enabled {
		if result.Pushed {
			pushTarget := plan.Push.Remote
			output.RowStatus(sec, "pushed", pushTarget, "success", useColor)
		} else if commitDryRun {
			sec.Row("%-16s%s (dry-run)", "push", plan.Push.Remote)
		}
	}

	sec.Close()
	return nil
}
