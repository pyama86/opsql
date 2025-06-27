package opsql

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
	"github.com/pyama86/opsql/internal/executor"
	"github.com/pyama86/opsql/internal/github"
	"github.com/pyama86/opsql/internal/slack"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute SQL operations",
	Long: `Run executes SQL operations with or without database changes.
By default, it executes operations and commits changes to the database.
Use --dry-run to execute in dry-run mode without making permanent changes.`,
	RunE: runRun,
}

func init() {
	runCmd.Flags().StringP("config", "c", "", "YAML configuration file path (required)")
	runCmd.Flags().BoolP("dry-run", "d", false, "Execute in dry-run mode without making permanent changes")
	runCmd.Flags().StringP("environment", "e", "", "Environment name (e.g., dev, staging, prod)")
	runCmd.Flags().String("github-repo", "", "GitHub repository (owner/repo)")
	runCmd.Flags().Int("github-pr", 0, "GitHub PR number")
	runCmd.Flags().String("slack-webhook", "", "Slack webhook URL (optional, can use SLACK_WEBHOOK_URL env)")

	_ = runCmd.MarkFlagRequired("config")
}

type RunConfig struct {
	ConfigFile   string
	DatabaseDSN  string
	DryRun       bool
	Environment  string
	GitHubRepo   string
	GitHubPR     int
	SlackWebhook string
}

func runRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	config, err := loadRunConfig(cmd)
	if err != nil {
		return err
	}

	def, err := definition.LoadDefinition(config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load definition: %w", err)
	}

	db, err := database.NewDatabase(config.DatabaseDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
		}
	}()

	var reports []definition.Report
	if config.DryRun {
		planExecutor := executor.NewPlanExecutor(db)
		reports, err = planExecutor.Execute(ctx, def)
	} else {
		applyExecutor := executor.NewApplyExecutor(db)
		reports, err = applyExecutor.Execute(ctx, def)
	}
	if err != nil {
		if config.DryRun {
			return fmt.Errorf("failed to execute dry run: %w", err)
		}
		return fmt.Errorf("failed to execute: %w", err)
	}

	if err := outputRunReports(reports); err != nil {
		return fmt.Errorf("failed to output reports: %w", err)
	}

	if err := sendRunGitHubComment(ctx, config, reports); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send GitHub comment: %v\n", err)
	}

	if err := sendRunSlackNotification(config, reports); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send Slack notification: %v\n", err)
	}

	return nil
}

func loadRunConfig(cmd *cobra.Command) (*RunConfig, error) {
	config := &RunConfig{}

	config.ConfigFile, _ = cmd.Flags().GetString("config")
	config.DryRun, _ = cmd.Flags().GetBool("dry-run")
	config.Environment, _ = cmd.Flags().GetString("environment")
	config.GitHubRepo, _ = cmd.Flags().GetString("github-repo")
	config.GitHubPR, _ = cmd.Flags().GetInt("github-pr")
	config.SlackWebhook, _ = cmd.Flags().GetString("slack-webhook")

	// Environment can also be set from OPSQL_ENVIRONMENT env var
	if config.Environment == "" {
		config.Environment = os.Getenv("OPSQL_ENVIRONMENT")
	}

	config.DatabaseDSN = os.Getenv("DATABASE_DSN")
	if config.DatabaseDSN == "" {
		return nil, fmt.Errorf("DATABASE_DSN environment variable is required")
	}

	return config, nil
}

func outputRunReports(reports []definition.Report) error {
	jsonData, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))
	return nil
}

func sendRunGitHubComment(ctx context.Context, config *RunConfig, reports []definition.Report) error {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("GITHUB_TOKEN") == "" {
		return nil
	}

	client := github.NewClient(config.GitHubRepo, config.GitHubPR)
	return client.PostCommentWithContext(ctx, reports, config.DryRun, config.Environment)
}

func sendRunSlackNotification(config *RunConfig, reports []definition.Report) error {
	webhookURL := config.SlackWebhook
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}

	if webhookURL == "" {
		return nil
	}

	client := slack.NewClient(webhookURL)
	return client.SendNotificationWithContext(reports, config.DryRun, config.Environment)
}