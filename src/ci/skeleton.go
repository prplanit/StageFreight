package ci

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// SkeletonVariant holds parsed metadata + content of a skeleton file.
type SkeletonVariant struct {
	Name     string   // "standard", "lightweight"
	Provider string   // "gitlab"
	Supports []string // facet subsystems this skeleton can run
	Content  []byte   // raw YAML
}

// ParseSkeleton extracts metadata from a skeleton file's header comments.
func ParseSkeleton(name, provider string, data []byte) (*SkeletonVariant, error) {
	sv := &SkeletonVariant{
		Name:     name,
		Provider: provider,
		Content:  data,
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			break
		}
		line = strings.TrimPrefix(line, "# ")
		if strings.HasPrefix(line, "supports: ") {
			parts := strings.Split(strings.TrimPrefix(line, "supports: "), ", ")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					sv.Supports = append(sv.Supports, p)
				}
			}
		}
	}

	return sv, nil
}

// SkeletonPath returns the repo-relative path for a skeleton variant.
func SkeletonPath(provider, variant string) string {
	return fmt.Sprintf("skeletons/%s/%s.yml", provider, variant)
}

// ValidateSkeleton checks if the active execution graph is compatible with a skeleton variant.
func ValidateSkeleton(graph *ExecutionGraph, variant *SkeletonVariant) (errors []string, warnings []string) {
	supported := make(map[string]bool)
	for _, s := range variant.Supports {
		supported[s] = true
	}

	// Active facet not supported by skeleton → error.
	for _, node := range graph.Nodes {
		if !supported[node.Subsystem] {
			errors = append(errors, fmt.Sprintf(
				"facet %q (subsystem %q) is active but skeleton %q does not support it",
				node.ID, node.Subsystem, variant.Name))
		}
	}

	// Skeleton supports facet that's inactive → info.
	activeSubsystems := make(map[string]bool)
	for _, node := range graph.Nodes {
		activeSubsystems[node.Subsystem] = true
	}
	for _, s := range variant.Supports {
		if !activeSubsystems[s] {
			warnings = append(warnings, fmt.Sprintf(
				"skeleton %q supports %q but no active facet uses it (will skip at runtime)",
				variant.Name, s))
		}
	}

	return errors, warnings
}
