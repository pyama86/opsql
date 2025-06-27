package slack

import (
	"fmt"
	"os"

	"github.com/pyama86/opsql/internal/definition"
	"github.com/slack-go/slack"
)

type Client struct {
	webhookURL string
}

func NewClient(webhookURL string) *Client {
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}
	return &Client{webhookURL: webhookURL}
}

func (c *Client) SendNotification(reports []definition.Report) error {
	return c.SendNotificationWithContext(reports, false, "")
}

func (c *Client) SendNotificationWithContext(reports []definition.Report, isDryRun bool, environment string) error {
	if c.webhookURL == "" {
		return fmt.Errorf("SLACK_WEBHOOK_URL is not set")
	}

	blocks := c.buildBlocksWithContext(reports, isDryRun, environment)
	msg := &slack.WebhookMessage{
		Blocks: &slack.Blocks{
			BlockSet: blocks,
		},
	}

	return slack.PostWebhook(c.webhookURL, msg)
}

func (c *Client) buildBlocks(reports []definition.Report) []slack.Block {
	return c.buildBlocksWithContext(reports, false, "")
}

func (c *Client) buildBlocksWithContext(reports []definition.Report, isDryRun bool, environment string) []slack.Block {
	passCount := 0
	failCount := 0

	for _, report := range reports {
		if report.Pass {
			passCount++
		} else {
			failCount++
		}
	}

	var blocks []slack.Block

	// Header block with context
	headerText := "🔧 opsql Execution Results"
	if isDryRun {
		headerText += " (Dry Run)"
	}
	if environment != "" {
		headerText += fmt.Sprintf(" [%s]", environment)
	}
	blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", headerText, false, false)))

	// Summary section
	summaryEmoji := "✅"
	if failCount > 0 {
		summaryEmoji = "❌"
	}

	summaryText := fmt.Sprintf("%s *Summary:* %d passed, %d failed", summaryEmoji, passCount, failCount)
	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", summaryText, false, false),
		nil, nil,
	))

	// Divider
	blocks = append(blocks, slack.NewDividerBlock())

	// Operation details
	for _, report := range reports {
		blocks = append(blocks, c.buildOperationBlock(report))
	}

	return blocks
}

func (c *Client) buildOperationBlock(report definition.Report) slack.Block {
	status := "✅ PASS"
	if !report.Pass {
		status = "❌ FAIL"
	}

	// Main section with operation info
	mainText := fmt.Sprintf("*%s* `%s`\n%s", status, report.ID, report.Description)

	var fields []*slack.TextBlockObject

	// Type field
	fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Type:*\n%s", report.Type), false, false))

	// Status field
	fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Status:*\n%s", report.Message), false, false))

	// SQL Query field
	if report.SQL != "" {
		fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Query:*\n```%s```", report.SQL), false, false))
	}

	// Result field for DML operations
	if report.Result != nil && report.Type != definition.TypeSelect {
		fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Affected Rows:*\n%v", report.Result), false, false))
	}

	sectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", mainText, false, false),
		fields,
		nil,
	)

	return sectionBlock
}
