package cmd

import (
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Documentation generation commands",
	Long:  "Generate reference documentation from code and config structs.",
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
