package release

import (
	"regexp"
	"sort"
	"strings"

	"github.com/PrPlanIT/StageFreight/src/config"
)

// ProcessedCommit is a commit that has been through the glossary pipeline.
// Raw evidence preserved; sanitized and presented forms derived.
type ProcessedCommit struct {
	Raw      Commit // original parsed commit
	Type     string // canonical type (after alias resolution)
	Priority int    // from glossary + breaking boost
	Breaking bool   // from any detection method
	Summary  string // sanitized + rewritten summary
	Included bool   // passes release_visible filter
}

// ProcessCommits runs the full glossary pipeline on parsed commits.
// Pipeline: normalize → detect breaking → sanitize → rewrite → score → rank.
func ProcessCommits(commits []Commit, glossary config.GlossaryConfig) []ProcessedCommit {
	var results []ProcessedCommit

	// Build alias lookup: alias → canonical type name
	aliasMap := buildAliasMap(glossary)

	for _, c := range commits {
		pc := ProcessedCommit{
			Raw:     c,
			Summary: c.Summary,
		}

		// 1. Normalize type via glossary
		pc.Type = resolveCanonicalType(c.Type, aliasMap, glossary)

		// 2. Detect breaking
		pc.Breaking = c.Breaking || isBreakingAlias(c.Type, glossary.Breaking)

		// 3. Look up priority and visibility
		if gt, ok := glossary.Types[pc.Type]; ok {
			pc.Priority = gt.Priority
			pc.Included = gt.ReleaseVisible
		}

		// Breaking boost
		if pc.Breaking {
			pc.Priority += glossary.Breaking.PriorityBoost
			if glossary.Breaking.ForceHighlight {
				pc.Included = true
			}
		}

		// 4. Sanitize summary
		pc.Summary = sanitizeSummary(pc.Summary, glossary.Filters)

		// 5. Apply rewrites
		pc.Summary = applyRewrites(pc.Summary, glossary.Rewrites)

		results = append(results, pc)
	}

	// 6. Sort by priority descending
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Priority > results[j].Priority
	})

	return results
}

// FormatHighlights renders processed commits as bulleted highlights.
// Respects release_visible and max_entries from the glossary/presentation.
func FormatHighlights(commits []ProcessedCommit, maxEntries int) string {
	var bullets []string

	for _, pc := range commits {
		if !pc.Included {
			continue
		}
		if maxEntries > 0 && len(bullets) >= maxEntries {
			break
		}

		bullet := "- "
		if pc.Raw.Scope != "" {
			bullet += pc.Raw.Scope + ": "
		}
		bullet += pc.Summary

		// Deduplicate
		dup := false
		for _, existing := range bullets {
			if existing == bullet {
				dup = true
				break
			}
		}
		if !dup {
			bullets = append(bullets, bullet)
		}
	}

	return strings.Join(bullets, "\n")
}

// buildAliasMap creates a lookup from alias → canonical type name.
func buildAliasMap(glossary config.GlossaryConfig) map[string]string {
	m := make(map[string]string)
	for name, gt := range glossary.Types {
		// The type name itself
		m[name] = name
		// Its aliases
		for _, alias := range gt.Aliases {
			m[alias] = name
		}
	}
	return m
}

// resolveCanonicalType resolves a raw commit type to its canonical form.
func resolveCanonicalType(rawType string, aliasMap map[string]string, glossary config.GlossaryConfig) string {
	rawType = strings.TrimSuffix(rawType, "!") // strip bang for lookup
	rawType = strings.ToLower(rawType)

	canonical, ok := aliasMap[rawType]
	if !ok {
		return rawType // unknown type, pass through
	}

	// Check if canonical type itself normalizes further
	if gt, ok := glossary.Types[canonical]; ok && gt.CanonicalAs != "" {
		return gt.CanonicalAs
	}

	return canonical
}

// isBreakingAlias checks if the raw type is a breaking-change alias.
func isBreakingAlias(rawType string, breaking config.BreakingConfig) bool {
	raw := strings.ToLower(strings.TrimSuffix(rawType, "!"))
	for _, alias := range breaking.Aliases {
		if raw == strings.ToLower(alias) {
			return true
		}
	}
	return false
}

// sanitizeSummary applies filter rules to a commit summary.
func sanitizeSummary(summary string, filters config.FilterConfig) string {
	// Strip phrases from summary
	for _, phrase := range filters.Summary.StripPhrases {
		summary = strings.ReplaceAll(summary, phrase, "")
	}

	// Strip regex patterns from summary
	for _, pattern := range filters.Summary.StripRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		summary = re.ReplaceAllString(summary, "")
	}

	// Normalize whitespace
	if filters.NormalizeWhitespace {
		summary = strings.Join(strings.Fields(summary), " ")
	}

	return strings.TrimSpace(summary)
}

// applyRewrites applies deterministic text transformations.
func applyRewrites(summary string, rewrites config.RewriteConfig) string {
	// Phrase rewrites
	for _, r := range rewrites.Phrases {
		summary = strings.ReplaceAll(summary, r.From, r.To)
	}

	// Regex rewrites
	for _, r := range rewrites.Regex {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		summary = re.ReplaceAllString(summary, r.Replace)
	}

	return summary
}
