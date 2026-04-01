package governance

import (
	"fmt"
	"strings"
)

// ExplainResolution returns a human-readable resolution chain.
// Shows: managed file source, local file presence, merge decisions, preset origins.
func ExplainResolution(managedPresent bool, localPresent bool, trace MergeTrace) string {
	var b strings.Builder

	b.WriteString("Config resolution:\n")

	if managedPresent {
		b.WriteString("  managed: present (.stagefreight/stagefreight-managed.yml)\n")
	} else {
		b.WriteString("  managed: absent\n")
	}

	if localPresent {
		b.WriteString("  local:   present (.stagefreight.yml)\n")
	} else {
		b.WriteString("  local:   absent\n")
	}

	if len(trace.Entries) == 0 {
		b.WriteString("  merge:   no entries\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("  merge:   %d entries\n", len(trace.Entries)))
	b.WriteString("\n")

	// Group by source.
	sources := map[string]int{}
	overrides := 0
	for _, e := range trace.Entries {
		sources[e.Source]++
		if e.Overridden {
			overrides++
		}
	}

	for src, count := range sources {
		b.WriteString(fmt.Sprintf("  source %-30s %d values\n", src, count))
	}
	if overrides > 0 {
		b.WriteString(fmt.Sprintf("  overrides:                          %d\n", overrides))
	}

	return b.String()
}

// ExplainTrace returns a detailed per-entry trace for debugging.
func ExplainTrace(trace MergeTrace) string {
	var b strings.Builder

	b.WriteString("Merge trace:\n")
	for _, e := range trace.Entries {
		line := fmt.Sprintf("  %-40s %-10s source=%-30s layer=%d",
			e.Path, e.Operation, e.Source, e.Layer)
		if e.SourceRef != "" {
			line += fmt.Sprintf(" ref=%s", e.SourceRef)
		}
		if e.Overridden {
			line += fmt.Sprintf(" (overridden by %s)", e.OverriddenBy)
		}
		b.WriteString(line + "\n")
	}

	return b.String()
}

// RenderEffective returns the merged config as YAML (before gating).
func RenderEffective(merged map[string]any) ([]byte, error) {
	return renderCanonical(merged)
}

// RenderGated returns the execution plan as human-readable output (after gating).
func RenderGated(plan ExecutionPlan) string {
	var b strings.Builder

	b.WriteString("Execution plan:\n")

	if len(plan.Enabled) > 0 {
		b.WriteString("\n  Enabled:\n")
		for _, f := range plan.Enabled {
			b.WriteString(fmt.Sprintf("    %-30s %s\n", f.Domain, f.Reason))
		}
	}

	if len(plan.Skipped) > 0 {
		b.WriteString("\n  Skipped:\n")
		for _, f := range plan.Skipped {
			b.WriteString(fmt.Sprintf("    %-30s %s\n", f.Domain, f.Reason))
		}
	}

	return b.String()
}
