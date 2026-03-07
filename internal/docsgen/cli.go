package docsgen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GenerateCLIReference walks the Cobra command tree and emits a complete
// CLI reference markdown document. Hidden commands are omitted.
func GenerateCLIReference(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString(generatedHeader)

	// Collect all visible commands (depth-first, sorted by full path).
	cmds := collectCommands(root)

	// Top-of-page command index.
	b.WriteString("## Command Index\n\n")
	for _, c := range cmds {
		path := fullPath(c)
		short := c.Short
		if short == "" {
			short = "—"
		}
		b.WriteString(fmt.Sprintf("- [`%s`](#%s) — %s\n", path, anchor("cli", path), short))
	}
	b.WriteString("\n---\n\n")

	// Per-command sections.
	for _, c := range cmds {
		b.WriteString(renderCommand(c))
	}

	return b.String()
}

// collectCommands returns all non-hidden commands sorted by full path.
func collectCommands(root *cobra.Command) []*cobra.Command {
	var result []*cobra.Command
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}
		result = append(result, cmd)
		subs := cmd.Commands()
		sort.Slice(subs, func(i, j int) bool {
			return subs[i].Name() < subs[j].Name()
		})
		for _, sub := range subs {
			walk(sub)
		}
	}
	walk(root)
	return result
}

// fullPath returns the full command path (e.g., "stagefreight docker build").
func fullPath(cmd *cobra.Command) string {
	return cmd.CommandPath()
}

func renderCommand(cmd *cobra.Command) string {
	var b strings.Builder
	path := fullPath(cmd)

	// Anchor + heading.
	b.WriteString(anchorTag("cli", path) + "\n")
	b.WriteString(fmt.Sprintf("### %s\n\n", path))

	// Deprecation notice.
	if cmd.Deprecated != "" {
		b.WriteString(fmt.Sprintf("> **Deprecated:** %s\n\n", cmd.Deprecated))
	}

	// Usage line.
	if cmd.Use != "" {
		b.WriteString(fmt.Sprintf("**Usage:** `%s`\n\n", path+" "+strings.TrimPrefix(cmd.Use, cmd.Name()+" ")))
	}

	// Aliases.
	if len(cmd.Aliases) > 0 {
		b.WriteString(fmt.Sprintf("**Aliases:** %s\n\n", strings.Join(cmd.Aliases, ", ")))
	}

	// Description.
	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}
	if desc != "" {
		b.WriteString(desc + "\n\n")
	}

	// Examples.
	if cmd.Example != "" {
		b.WriteString("**Examples:**\n\n```\n" + cmd.Example + "\n```\n\n")
	}

	// Local flags table.
	localFlags := collectFlags(cmd.LocalFlags(), cmd.InheritedFlags())
	if len(localFlags) > 0 {
		b.WriteString("**Flags:**\n\n")
		b.WriteString(flagTable(localFlags))
		b.WriteString("\n")
	}

	// Inherited flags table.
	inheritedFlags := collectInheritedFlags(cmd.InheritedFlags())
	if len(inheritedFlags) > 0 {
		b.WriteString("**Inherited flags:**\n\n")
		b.WriteString(flagTable(inheritedFlags))
		b.WriteString("\n")
	}

	// Subcommands list.
	subs := visibleSubcommands(cmd)
	if len(subs) > 0 {
		b.WriteString("**Subcommands:**\n\n")
		for _, sub := range subs {
			subPath := fullPath(sub)
			short := sub.Short
			if short == "" {
				short = "—"
			}
			b.WriteString(fmt.Sprintf("- [`%s`](#%s) — %s\n", sub.Name(), anchor("cli", subPath), short))
		}
		b.WriteString("\n")
	}

	// See also: parent + children (for groups) or parent + siblings (for leaves).
	if sa := renderSeeAlso(cmd); sa != "" {
		b.WriteString(sa)
	}

	b.WriteString("---\n\n")
	return b.String()
}

// renderSeeAlso generates a "See also" line with parent/child/sibling navigation.
// Groups show parent + children. Leaves show parent + siblings.
func renderSeeAlso(cmd *cobra.Command) string {
	var links []string

	parent := cmd.Parent()
	if parent != nil {
		links = append(links, fmt.Sprintf("[`%s`](#%s)", fullPath(parent), anchor("cli", fullPath(parent))))
	}

	children := visibleSubcommands(cmd)
	if len(children) > 0 {
		// Group command: link to children.
		for _, child := range children {
			links = append(links, fmt.Sprintf("[`%s`](#%s)", fullPath(child), anchor("cli", fullPath(child))))
		}
	} else if parent != nil {
		// Leaf command: link to siblings.
		for _, sib := range visibleSubcommands(parent) {
			if sib == cmd {
				continue
			}
			links = append(links, fmt.Sprintf("[`%s`](#%s)", fullPath(sib), anchor("cli", fullPath(sib))))
		}
	}

	if len(links) == 0 {
		return ""
	}
	return "**See also:** " + strings.Join(links, " · ") + "\n\n"
}

// collectFlags extracts local-only flags (excluding inherited), sorted by name.
func collectFlags(local, inherited *pflag.FlagSet) []flagRow {
	inheritedNames := map[string]bool{}
	if inherited != nil {
		inherited.VisitAll(func(f *pflag.Flag) {
			inheritedNames[f.Name] = true
		})
	}

	var rows []flagRow
	if local != nil {
		local.VisitAll(func(f *pflag.Flag) {
			if f.Hidden || inheritedNames[f.Name] {
				return
			}
			name := "--" + f.Name
			if f.Shorthand != "" {
				name = "-" + f.Shorthand + ", " + name
			}
			rows = append(rows, flagRow{
				Name:        name,
				Type:        f.Value.Type(),
				Default:     formatDefault(f),
				Description: f.Usage,
			})
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

// collectInheritedFlags extracts inherited flags, sorted by name.
func collectInheritedFlags(inherited *pflag.FlagSet) []flagRow {
	var rows []flagRow
	if inherited != nil {
		inherited.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			name := "--" + f.Name
			if f.Shorthand != "" {
				name = "-" + f.Shorthand + ", " + name
			}
			rows = append(rows, flagRow{
				Name:        name,
				Type:        f.Value.Type(),
				Default:     formatDefault(f),
				Description: f.Usage,
			})
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

func formatDefault(f *pflag.Flag) string {
	if f.DefValue == "" || f.DefValue == "false" || f.DefValue == "0" || f.DefValue == "[]" {
		return "—"
	}
	return "`" + f.DefValue + "`"
}

func visibleSubcommands(cmd *cobra.Command) []*cobra.Command {
	var subs []*cobra.Command
	for _, sub := range cmd.Commands() {
		if !sub.Hidden {
			subs = append(subs, sub)
		}
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })
	return subs
}
