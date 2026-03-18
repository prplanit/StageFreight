package pipeline

import (
	"fmt"
	"time"

	"github.com/PrPlanIT/StageFreight/src/output"
)

// renderSummary writes the summary table from accumulated PhaseResults.
func renderSummary(pc *PipelineContext) {
	if len(pc.Results) == 0 {
		return
	}

	totalElapsed := time.Since(pc.PipelineStart)
	overallStatus := "success"

	sumSec := output.NewSection(pc.Writer, "Summary", 0, pc.Color)

	var failure *FailureDetail
	for _, r := range pc.Results {
		// Skip banner from summary — it's infrastructure, not a reportable phase
		if r.Name == "banner" {
			continue
		}
		// Skip dry-run gate when it didn't activate
		if r.Name == "dry-run" && r.Status == "skipped" {
			continue
		}

		if r.Status == "failed" {
			overallStatus = "failed"
			if r.Failure != nil && failure == nil {
				failure = r.Failure
			}
		}

		if r.Summary != "" {
			output.SummaryRow(pc.Writer, r.Name, r.Status, r.Summary, pc.Color)
		}
	}

	sumSec.Separator()
	output.SummaryTotal(pc.Writer, totalElapsed, overallStatus, pc.Color)
	sumSec.Close()

	// Exit Reason section — operator-facing failure context
	if failure != nil {
		renderExitReason(pc, failure)
	}
}

// renderExitReason renders the operator-facing Exit Reason box.
// Single-line when command + exit code + reason fit in ~80 chars, two-line otherwise.
func renderExitReason(pc *PipelineContext, f *FailureDetail) {
	w := pc.Writer

	exitSuffix := ""
	if f.ExitCode != 0 {
		exitSuffix = fmt.Sprintf(" (exit %d)", f.ExitCode)
	}

	oneLine := fmt.Sprintf("%s%s — %s", f.Command, exitSuffix, f.Reason)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "── Exit Reason ────────────────────────────────────────────")
	if len(oneLine) <= 80 {
		fmt.Fprintf(w, "│ %s\n", oneLine)
	} else {
		fmt.Fprintf(w, "│ %s%s\n", f.Command, exitSuffix)
		fmt.Fprintf(w, "│ reason: %s\n", f.Reason)
	}
	fmt.Fprintln(w, "└──────────────────────────────────────────────────────────────")

	// Dump stderr in CI collapsed section or when verbose
	if f.Stderr != "" {
		if pc.CI {
			output.SectionStartCollapsed(pc.Writer, "sf_exit_stderr", "Failure Stderr")
			fmt.Fprint(w, f.Stderr)
			output.SectionEnd(pc.Writer, "sf_exit_stderr")
		} else if pc.Verbose {
			fmt.Fprintln(w)
			fmt.Fprint(w, f.Stderr)
		}
	}
}
