package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v73/github"
	"github.com/pyama86/opsql/internal/definition"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
	repo   string
	pr     int
}

func NewClient(repo string, pr int) *Client {
	// Try GitHub App authentication first
	if client := newGitHubAppClient(); client != nil {
		return &Client{
			client: client,
			repo:   repo,
			pr:     pr,
		}
	}

	// Fallback to personal access token
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &Client{
		client: client,
		repo:   repo,
		pr:     pr,
	}
}

func (c *Client) PostComment(ctx context.Context, reports []definition.Report) error {
	return c.PostCommentWithContext(ctx, reports, false, "")
}

func (c *Client) PostCommentWithContext(ctx context.Context, reports []definition.Report, isDryRun bool, environment string) error {
	if c.client == nil {
		return fmt.Errorf("GitHub authentication not configured (GITHUB_TOKEN or GitHub App credentials required)")
	}

	if c.repo == "" {
		c.repo = os.Getenv("GITHUB_REPOSITORY")
	}

	if c.pr == 0 {
		c.pr = extractPRNumber()
	}

	if c.repo == "" || c.pr == 0 {
		log.Printf("GITHUB_REPOSITORY or GITHUB_PR environment variables are not set, skipping GitHub comment\n")
		return nil
	}

	parts := strings.Split(c.repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", c.repo)
	}

	owner, repoName := parts[0], parts[1]
	comment := formatCommentWithContext(reports, isDryRun, environment)

	// Try to find and update existing opsql comment
	existingComment, err := c.findExistingOpsqlComment(ctx, owner, repoName, environment)
	if err != nil {
		return fmt.Errorf("failed to search for existing comments: %w", err)
	}

	if existingComment != nil {
		// Update existing comment
		_, _, err = c.client.Issues.EditComment(ctx, owner, repoName, *existingComment.ID, &github.IssueComment{
			Body: &comment,
		})
		if err != nil {
			return fmt.Errorf("failed to update existing comment: %w", err)
		}
	} else {
		// Create new comment
		_, _, err = c.client.Issues.CreateComment(ctx, owner, repoName, c.pr, &github.IssueComment{
			Body: &comment,
		})
		if err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}
	}

	return nil
}

func formatComment(reports []definition.Report) string {
	return formatCommentWithContext(reports, false, "")
}

func formatCommentWithContext(reports []definition.Report, isDryRun bool, environment string) string {
	var buf strings.Builder
	title := "## "
	if environment != "" {
		title += fmt.Sprintf("[%s] ", environment)
	}
	title += "opsql Execution Results"
	if isDryRun {
		title += " (Dry Run)"
	}
	buf.WriteString(title + "\n\n")

	passCount := 0
	failCount := 0

	for _, report := range reports {
		if report.Pass {
			passCount++
		} else {
			failCount++
		}
	}

	buf.WriteString(fmt.Sprintf("**Summary:** %d passed, %d failed\n\n", passCount, failCount))

	for _, report := range reports {
		status := "✅"
		if !report.Pass {
			status = "❌"
		}

		buf.WriteString(fmt.Sprintf("### %s %s - %s\n", status, report.ID, report.Description))
		buf.WriteString(fmt.Sprintf("**Type:** %s\n", report.Type))
		buf.WriteString(fmt.Sprintf("**Status:** %s\n", report.Message))

		// Add SQL query
		if report.SQL != "" {
			buf.WriteString("**Query:**\n```sql\n")
			buf.WriteString(report.SQL)
			buf.WriteString("\n```\n")
		}

		if report.Type == definition.TypeSelect && report.Result != nil {
			if rows, ok := report.Result.([]map[string]interface{}); ok && len(rows) > 0 {
				buf.WriteString("**Result:**\n```json\n")
				jsonData, _ := json.MarshalIndent(rows, "", "  ")
				buf.WriteString(string(jsonData))
				buf.WriteString("\n```\n")
			}
		} else if report.Result != nil {
			buf.WriteString(fmt.Sprintf("**Affected Rows:** %v\n", report.Result))
		}

		buf.WriteString("\n")
	}

	return buf.String()
}

func extractPRNumber() int {
	ref := os.Getenv("GITHUB_REF")
	if ref == "" {
		return 0
	}

	if strings.HasPrefix(ref, "refs/pull/") && strings.HasSuffix(ref, "/merge") {
		parts := strings.Split(ref, "/")
		if len(parts) >= 3 {
			if num, err := strconv.Atoi(parts[2]); err == nil {
				return num
			}
		}
	}

	return 0
}

// newGitHubAppClient creates a GitHub client using GitHub App authentication
func newGitHubAppClient() *github.Client {
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		appID = os.Getenv("GITHUB_APP_CLIENT_ID")
	}
	installationID := os.Getenv("GITHUB_APP_INSTALLATION_ID")
	privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	privateKeyContent := os.Getenv("GITHUB_APP_PRIVATE_KEY")

	if appID == "" || installationID == "" {
		log.Printf("GITHUB_APP_ID and GITHUB_APP_INSTALLATION_ID must be set for GitHub App authentication\n")
		return nil
	}

	if privateKeyPath == "" && privateKeyContent == "" {
		log.Printf("Either GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_APP_PRIVATE_KEY must be set for GitHub App authentication\n")
		return nil
	}

	var privateKeyData []byte
	var err error

	if privateKeyContent != "" {
		privateKeyData = []byte(privateKeyContent)
	} else {
		privateKeyData, err = os.ReadFile(privateKeyPath)
		if err != nil {
			log.Printf("failed to read private key from %s: %v\n", privateKeyPath, err)
			return nil
		}
	}

	installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		log.Printf("invalid GITHUB_APP_INSTALLATION_ID: %v\n", err)
		return nil
	}

	appTransport, err := NewAppsTransport(http.DefaultTransport, appID, privateKeyData)
	if err != nil {
		log.Printf("failed to create GitHub App transport: %v\n", err)
		return nil
	}

	client := github.NewClient(
		&http.Client{
			Transport: appTransport,
			Timeout:   time.Second * 30,
		},
	)

	token, _, err := client.Apps.CreateInstallationToken(
		context.Background(),
		installationIDInt,
		&github.InstallationTokenOptions{})
	if err != nil {
		log.Printf("failed to create installation token: %v\n", err)
		return nil
	}

	return github.NewClient(nil).WithAuthToken(
		token.GetToken(),
	)
}

// findExistingOpsqlComment searches for existing opsql comments in the PR
func (c *Client) findExistingOpsqlComment(ctx context.Context, owner, repoName, environment string) (*github.IssueComment, error) {
	// List all comments for the PR
	comments, _, err := c.client.Issues.ListComments(ctx, owner, repoName, c.pr, &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, err
	}

	// Build the expected comment prefix to identify opsql comments
	expectedPrefix := "## "
	if environment != "" {
		expectedPrefix += fmt.Sprintf("[%s] ", environment)
	}
	expectedPrefix += "opsql Execution Results"

	// Search for existing opsql comment
	for _, comment := range comments {
		if comment.Body != nil && strings.HasPrefix(*comment.Body, expectedPrefix) {
			return comment, nil
		}
	}

	return nil, nil
}
