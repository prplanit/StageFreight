package ci

import (
	"fmt"

	"github.com/PrPlanIT/StageFreight/src/config"
)

// ExecutionGraph is the canonical, provider-agnostic execution model.
// Computed from config. Inspectable. Testable. The portable product.
type ExecutionGraph struct {
	Nodes []GraphNode
}

// GraphNode represents one unit of work in the execution graph.
type GraphNode struct {
	ID        string   // "build", "security", "reconcile"
	Subsystem string   // what `ci run <x>` dispatches to
	Needs     []string // explicit dependency IDs (not position-based)
	NeedsDinD bool
	Active    bool     // from facet detection
	Reason    string   // why active ("builds configured") or inactive ("no builds")
}

// BuildGraph computes the canonical execution graph from effective config.
// Provider-agnostic. No heuristics — only config presence.
func BuildGraph(cfg *config.Config) *ExecutionGraph {
	facets := DetectActive(cfg)
	active := make(map[string]bool)
	for _, f := range facets {
		active[f.Name] = true
	}

	// Build all possible nodes with reasons.
	nodes := []GraphNode{
		{
			ID:        "validate",
			Subsystem: "validate",
			Needs:     nil,
			NeedsDinD: false,
			Active:    active["validate"],
			Reason:    reason(active["validate"], "lint configured", "no lint config"),
		},
		{
			ID:        "deps",
			Subsystem: "deps",
			Needs:     nil,
			NeedsDinD: true,
			Active:    active["deps"],
			Reason:    reason(active["deps"], "dependency update enabled", "dependency update disabled"),
		},
		{
			ID:        "build",
			Subsystem: "build",
			Needs:     nil,
			NeedsDinD: true,
			Active:    active["build"],
			Reason:    reason(active["build"], "builds configured", "no builds configured"),
		},
		{
			ID:        "security",
			Subsystem: "security",
			Needs:     needsIf("build", active["build"]),
			NeedsDinD: true,
			Active:    active["security"],
			Reason:    reason(active["security"], "security scanning enabled", "security scanning disabled"),
		},
		{
			ID:        "release",
			Subsystem: "release",
			Needs:     releaseNeeds(active),
			NeedsDinD: false,
			Active:    active["release"],
			Reason:    reason(active["release"], "release enabled", "release disabled"),
		},
		{
			ID:        "reconcile",
			Subsystem: "reconcile",
			Needs:     nil,
			NeedsDinD: false,
			Active:    active["gitops-reconcile"] || active["governance-reconcile"],
			Reason:    reconcileReason(active),
		},
		{
			ID:        "docs",
			Subsystem: "docs",
			Needs:     docsNeeds(active),
			NeedsDinD: false,
			Active:    active["docs"],
			Reason:    reason(active["docs"], "docs enabled", "docs disabled"),
		},
	}

	// Filter to active only.
	var result []GraphNode
	for _, n := range nodes {
		if n.Active {
			// Prune needs to only reference active nodes.
			n.Needs = pruneInactive(n.Needs, nodes)
			result = append(result, n)
		}
	}

	return &ExecutionGraph{Nodes: result}
}

// ActiveNodeIDs returns the IDs of all active nodes.
func (g *ExecutionGraph) ActiveNodeIDs() []string {
	ids := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		ids[i] = n.ID
	}
	return ids
}

// NeedsDinD returns true if any active node requires Docker-in-Docker.
func (g *ExecutionGraph) NeedsDinD() bool {
	for _, n := range g.Nodes {
		if n.NeedsDinD {
			return true
		}
	}
	return false
}

// Render returns a human-readable representation of the graph.
func (g *ExecutionGraph) Render() string {
	var out string
	for _, n := range g.Nodes {
		deps := ""
		if len(n.Needs) > 0 {
			deps = fmt.Sprintf(" (needs: %v)", n.Needs)
		}
		dind := ""
		if n.NeedsDinD {
			dind = " [dind]"
		}
		out += fmt.Sprintf("  %-16s ci run %-12s %s%s%s\n", n.ID, n.Subsystem, n.Reason, deps, dind)
	}
	return out
}

// --- dependency helpers ---

func needsIf(id string, active bool) []string {
	if active {
		return []string{id}
	}
	return nil
}

// releaseNeeds: build if active, security if active.
func releaseNeeds(active map[string]bool) []string {
	var needs []string
	if active["build"] {
		needs = append(needs, "build")
	}
	if active["security"] {
		needs = append(needs, "security")
	}
	return needs
}

// docsNeeds: release if present → else reconcile if present → else build if present → else none.
func docsNeeds(active map[string]bool) []string {
	if active["release"] {
		return []string{"release"}
	}
	if active["gitops-reconcile"] || active["governance-reconcile"] {
		return []string{"reconcile"}
	}
	if active["build"] {
		return []string{"build"}
	}
	return nil
}

func reconcileReason(active map[string]bool) string {
	switch {
	case active["gitops-reconcile"]:
		return "gitops cluster configured"
	case active["governance-reconcile"]:
		return "governance clusters configured"
	default:
		return "no reconcile target"
	}
}

func reason(active bool, yes, no string) string {
	if active {
		return yes
	}
	return no
}

// pruneInactive removes dependency IDs that reference inactive nodes.
func pruneInactive(needs []string, all []GraphNode) []string {
	activeSet := make(map[string]bool)
	for _, n := range all {
		if n.Active {
			activeSet[n.ID] = true
		}
	}
	var pruned []string
	for _, id := range needs {
		if activeSet[id] {
			pruned = append(pruned, id)
		}
	}
	return pruned
}
