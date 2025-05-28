package opsql

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "opsql",
	Short: "A CLI tool for managing operational SQL with dry-run and automation features",
	Long: `opsql is a CLI tool that helps manage operational SQL operations with YAML definitions.
It provides dry-run capabilities, assertion validation, and integration with GitHub and Slack.

Features:
- Plan mode (dry-run) for safe SQL execution preview
- Apply mode for actual SQL execution with validation
- GitHub PR comment integration
- Slack notification support
- YAML-based operation definitions`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
}
