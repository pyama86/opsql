package opsql

import (
	"context"
	"fmt"

	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
	"github.com/pyama86/opsql/internal/executor"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Execute SQL operations with actual database changes",
	Long: `Apply executes SQL operations with actual database changes.
For SELECT operations, it retrieves results and validates against expected values.
For DML operations, it commits changes to the database after validation.
If any assertion fails, the process exits with code 1.`,
	RunE: runApply,
}

func init() {
	applyCmd.Flags().StringP("config", "c", "", "YAML configuration file path (required)")

	applyCmd.MarkFlagRequired("config")
}

func runApply(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	config, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	def, err := definition.LoadDefinition(config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load definition: %w", err)
	}

	db, err := database.NewDatabase(config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	executor := executor.NewApplyExecutor(db)
	reports, err := executor.Execute(ctx, def)
	if err != nil {
		return fmt.Errorf("failed to execute apply: %w", err)
	}

	if err := outputReports(reports); err != nil {
		return fmt.Errorf("failed to output reports: %w", err)
	}

	return nil
}
