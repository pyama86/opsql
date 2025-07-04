package opsql

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	runCmd.Flags().StringSliceP("config", "c", []string{}, "YAML configuration file paths (required, can specify multiple)")
	runCmd.Flags().BoolP("dry-run", "d", false, "Execute in dry-run mode without making permanent changes")
	runCmd.Flags().StringP("environment", "e", "", "Environment name (e.g., dev, staging, prod)")
	runCmd.Flags().String("github-repo", "", "GitHub repository (owner/repo)")
	runCmd.Flags().Int("github-pr", 0, "GitHub PR number")
	runCmd.Flags().String("slack-webhook", "", "Slack webhook URL (optional, can use SLACK_WEBHOOK_URL env)")

	_ = runCmd.MarkFlagRequired("config")
}

type RunConfig struct {
	ConfigFiles  []string
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

	def, err := definition.LoadDefinitions(config.ConfigFiles)
	if err != nil {
		definitionErr := fmt.Errorf("failed to load definition: %w", err)
		sendNotifications(ctx, config, nil, definitionErr)
		return definitionErr
	}

	db, err := database.NewDatabase(config.DatabaseDSN)
	if err != nil {
		dbErr := fmt.Errorf("failed to connect to database: %w", err)
		sendNotifications(ctx, config, nil, dbErr)
		return dbErr
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
		}
	}()

	var reports []definition.Report
	var executionErr error
	if config.DryRun {
		planExecutor := executor.NewPlanExecutor(db)
		reports, executionErr = planExecutor.Execute(ctx, def)
	} else {
		applyExecutor := executor.NewApplyExecutor(db)
		reports, executionErr = applyExecutor.Execute(ctx, def)
	}

	// Always output reports and send notifications, even on failure
	if len(reports) > 0 {
		if err := outputRunReports(reports); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to output reports: %v\n", err)
		}
	}

	// Send notifications regardless of whether we have reports
	sendNotifications(ctx, config, reports, executionErr)

	// Return the original execution error if it occurred
	if executionErr != nil {
		if config.DryRun {
			return fmt.Errorf("failed to execute dry run: %w", executionErr)
		}
		return fmt.Errorf("failed to execute: %w", executionErr)
	}

	return nil
}

func loadRunConfig(cmd *cobra.Command) (*RunConfig, error) {
	config := &RunConfig{}

	config.ConfigFiles, _ = cmd.Flags().GetStringSlice("config")
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

func sendRunGitHubCommentWithError(ctx context.Context, config *RunConfig, reports []definition.Report, executionErr error) error {
	client := github.NewClient(config.GitHubRepo, config.GitHubPR)
	if client == nil {
		log.Printf("GitHub client not configured, skipping comment\n")
		return nil // GitHub client not configured, skip sending comment
	}
	return client.PostCommentWithContextAndError(ctx, reports, config.DryRun, config.Environment, executionErr)
}

func sendRunSlackNotificationWithError(config *RunConfig, reports []definition.Report, executionErr error) error {
	webhookURL := config.SlackWebhook
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}

	if webhookURL == "" {
		return nil
	}

	client := slack.NewClient(webhookURL)
	return client.SendNotificationWithContextAndError(reports, config.DryRun, config.Environment, executionErr)
}

// sendNotifications sends notifications to both Slack and GitHub
func sendNotifications(ctx context.Context, config *RunConfig, reports []definition.Report, err error) {
	if err := sendRunGitHubCommentWithError(ctx, config, reports, err); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send GitHub comment: %v\n", err)
	}

	if err := sendRunSlackNotificationWithError(config, reports, err); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send Slack notification: %v\n", err)
	}
}
