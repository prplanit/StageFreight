package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var glossaryJSON bool

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "Show the repo's change-language conventions",
	Long: `Display the glossary of commit types, aliases, and release visibility
defined in .stagefreight.yml.

This is the shared semantic model used by commit authoring, tag planning,
and release rendering. Use --json for machine-readable output.`,
	RunE: runGlossary,
}

func init() {
	glossaryCmd.Flags().BoolVar(&glossaryJSON, "json", false, "output as JSON")

	rootCmd.AddCommand(glossaryCmd)
}

func runGlossary(cmd *cobra.Command, args []string) error {
	if glossaryJSON {
		return renderGlossaryJSON()
	}
	return renderGlossaryHuman()
}

func renderGlossaryJSON() error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg.Glossary)
}

func renderGlossaryHuman() error {
	g := cfg.Glossary

	// Collect and sort types by priority descending
	type typeEntry struct {
		name     string
		aliases  string
		priority int
		visible  string
	}

	var entries []typeEntry
	for name, gt := range g.Types {
		aliases := strings.Join(gt.Aliases, ", ")
		if aliases == "" {
			aliases = "-"
		}
		visible := "no"
		if gt.ReleaseVisible {
			visible = "yes"
		}
		entries = append(entries, typeEntry{
			name:     name,
			aliases:  aliases,
			priority: gt.Priority,
			visible:  visible,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].priority > entries[j].priority
	})

	// Print table
	fmt.Printf("  %-12s %-16s %-10s %s\n", "Type", "Aliases", "Priority", "Visible")
	for _, e := range entries {
		fmt.Printf("  %-12s %-16s %-10d %s\n", e.name, e.aliases, e.priority, e.visible)
	}

	// Breaking
	fmt.Println()
	breakAliases := strings.Join(g.Breaking.Aliases, ", ")
	fmt.Printf("  Breaking: %s", breakAliases)
	if g.Breaking.BangSuffix {
		fmt.Print(" (or ! suffix on any type)")
	}
	fmt.Println()

	// Syntax examples
	fmt.Println()
	fmt.Println("  Syntax: stagefreight commit <type|alias>[!] [scope] \"message\"")
	fmt.Println("  Example: stagefreight commit f \"add release planner\"")
	fmt.Println("  Example: stagefreight commit fx \"(ci)\" \"fix pipeline race\"")
	fmt.Println("  Example: stagefreight commit b feat \"remove v1 API support\"")

	return nil
}
