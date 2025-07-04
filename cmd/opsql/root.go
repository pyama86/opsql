package opsql

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "opsql",
	SilenceUsage: true,
	Short:        "A CLI tool for managing operational SQL with dry-run and automation features",
	Long: `opsql is a CLI tool that helps manage operational SQL operations with YAML definitions.
It provides dry-run capabilities, assertion validation, and integration with GitHub and Slack.

Features:
- Run mode with --dry-run flag for safe SQL execution preview
- Direct execution for actual SQL operations with validation
- Environment-specific execution with --environment flag
- GitHub PR comment integration
- Slack notification support
- YAML-based operation definitions`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// .envファイルを読み込み（存在しない場合は無視）
	_ = godotenv.Load()

	rootCmd.AddCommand(runCmd)
}
