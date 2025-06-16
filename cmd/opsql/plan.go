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

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Execute SQL operations in dry-run mode",
	Long: `Plan executes SQL operations in dry-run mode without making permanent changes.
For SELECT operations, it retrieves results and validates against expected values.
For DML operations, it executes within a transaction that is always rolled back.`,
	RunE: runPlan,
}

func init() {
	planCmd.Flags().StringP("config", "c", "", "YAML configuration file path (required)")
	planCmd.Flags().String("github-repo", "", "GitHub repository (owner/repo)")
	planCmd.Flags().Int("github-pr", 0, "GitHub PR number")
	planCmd.Flags().String("slack-webhook", "", "Slack webhook URL (optional, can use SLACK_WEBHOOK_URL env)")

	_ = planCmd.MarkFlagRequired("config")
}

func runPlan(cmd *cobra.Command, args []string) error {
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
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
		}
	}()

	executor := executor.NewPlanExecutor(db)
	reports, err := executor.Execute(ctx, def)
	if err != nil {
		return fmt.Errorf("failed to execute plan: %w", err)
	}

	if err := outputReports(reports); err != nil {
		return fmt.Errorf("failed to output reports: %w", err)
	}

	if err := sendGitHubComment(ctx, config, reports); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send GitHub comment: %v\n", err)
	}

	if err := sendSlackNotification(config, reports); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send Slack notification: %v\n", err)
	}

	return nil
}

type Config struct {
	ConfigFile   string
	DatabaseURL  string
	GitHubRepo   string
	GitHubPR     int
	SlackWebhook string
}

func loadConfig(cmd *cobra.Command) (*Config, error) {
	config := &Config{}

	config.ConfigFile, _ = cmd.Flags().GetString("config")
	config.GitHubRepo, _ = cmd.Flags().GetString("github-repo")
	config.GitHubPR, _ = cmd.Flags().GetInt("github-pr")
	config.SlackWebhook, _ = cmd.Flags().GetString("slack-webhook")

	config.DatabaseURL = os.Getenv("DATABASE_URL")
	if config.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	return config, nil
}

func outputReports(reports []definition.Report) error {
	jsonData, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))
	return nil
}

func sendGitHubComment(ctx context.Context, config *Config, reports []definition.Report) error {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("GITHUB_TOKEN") == "" {
		return nil
	}

	client := github.NewClient(config.GitHubRepo, config.GitHubPR)
	return client.PostComment(ctx, reports)
}

func sendSlackNotification(config *Config, reports []definition.Report) error {
	webhookURL := config.SlackWebhook
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}

	if webhookURL == "" {
		return nil
	}

	client := slack.NewClient(webhookURL)
	return client.SendNotification(reports)
}
