package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/pyama86/opsql/internal/definition"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
	repo   string
	pr     int
}

func NewClient(repo string, pr int) *Client {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return &Client{repo: repo, pr: pr}
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
		return fmt.Errorf("GITHUB_TOKEN is not set")
	}

	if c.repo == "" {
		c.repo = os.Getenv("GITHUB_REPOSITORY")
	}

	if c.pr == 0 {
		c.pr = extractPRNumber()
	}

	if c.repo == "" || c.pr == 0 {
		return fmt.Errorf("GitHub repository or PR number not specified")
	}

	parts := strings.Split(c.repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", c.repo)
	}

	owner, repoName := parts[0], parts[1]
	comment := formatCommentWithContext(reports, isDryRun, environment)

	_, _, err := c.client.Issues.CreateComment(ctx, owner, repoName, c.pr, &github.IssueComment{
		Body: &comment,
	})

	if err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
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
