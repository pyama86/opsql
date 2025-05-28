# opsql - Operational SQL Automation Tool

opsql is a CLI tool that helps manage operational SQL operations with YAML definitions. It provides dry-run capabilities, assertion validation, and integration with GitHub and Slack for safe database operations.

## Features

- **Plan Mode (Dry-run)**: Execute SQL operations without permanent changes
- **Apply Mode**: Execute SQL operations with actual database changes
- **YAML-based Configuration**: Define operations in structured, reviewable format
- **Assertion Validation**: Validate results against expected values
- **GitHub Integration**: Automatic PR comments with execution results
- **Slack Notifications**: Rich block-based notifications
- **Template Support**: Use parameters in SQL with Go text/template
- **Multi-database Support**: PostgreSQL and MySQL compatible

## Installation

### Using Go Install

```bash
go install github.com/pyama86/opsql@latest
```

### From Source

```bash
git clone https://github.com/pyama86/opsql.git
cd opsql
go build -o opsql main.go
```

## Quick Start

### 1. Create a YAML Configuration

```yaml
version: 1
params:
  cutoff_date: "2025-01-01"
  target_user_ids: "1,2,3,4,5"
operations:
  - id: check_target_users
    description: "Check specific users before processing"
    type: select
    sql: |
      SELECT id, email, status
      FROM users
      WHERE id IN ({{ .params.target_user_ids }})
      ORDER BY id
    expected:
      - id: 1
        email: "user1@example.com"
        status: "active"
      - id: 2
        email: "user2@example.com"
        status: "active"

  - id: update_users_by_id_list
    description: "Update specific users to inactive status"
    type: update
    sql: |
      UPDATE users
      SET status = 'inactive', updated_at = NOW()
      WHERE id IN ({{ .params.target_user_ids }})
        AND status = 'active'
    expected_changes:
      update: 3
```

### 2. Set Environment Variables

You can set environment variables in two ways:

**Option A: Using .env file (recommended)**

```bash
# Copy the example file and edit it
cp .env.example .env
# Edit .env file with your actual values
```

**Option B: Export environment variables**

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/dbname"
# Optional: for GitHub integration
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
# Optional: for Slack notifications
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/xxx/yyy/zzz"
```

### 3. Run Plan (Dry-run)

```bash
opsql plan --config operations.yaml
```

### 4. Apply Changes

```bash
opsql apply --config operations.yaml
```

## Command Reference

### plan

Execute SQL operations in dry-run mode without making permanent changes.

```bash
opsql plan [flags]
```

**Flags:**

- `-c, --config string`: YAML configuration file path (required)
- `--github-repo string`: GitHub repository (owner/repo)
- `--github-pr int`: GitHub PR number
- `--slack-webhook string`: Slack webhook URL

**Examples:**

```bash
# Basic plan execution
opsql plan --config operations.yaml

# With GitHub PR integration
opsql plan --config operations.yaml --github-repo myorg/myrepo --github-pr 123

# With Slack notification
opsql plan --config operations.yaml --slack-webhook https://hooks.slack.com/services/xxx
```

### apply

Execute SQL operations with actual database changes.

```bash
opsql apply [flags]
```

**Flags:**

- `-c, --config string`: YAML configuration file path (required)

**Examples:**

```bash
# Apply changes
opsql apply --config operations.yaml
```

## YAML Configuration Reference

### Structure

```yaml
version: 1 # Configuration version (required)
params: # Template parameters (optional)
  key: "value"
operations: # List of operations (required)
  - id: "operation_id" # Unique identifier (required)
    description: "desc" # Human-readable description (required)
    type: "select|insert|update|delete" # Operation type (required)
    sql: | # SQL statement (required)
      SELECT * FROM table
    expected: # For SELECT operations (required for SELECT)
      - column: value
    expected_changes: # For DML operations (required for DML)
      insert|update|delete: count
```

### Operation Types

#### SELECT Operations

```yaml
- id: get_users
  description: "Get active users"
  type: select
  sql: "SELECT id, email FROM users WHERE status = 'active'"
  expected:
    - id: 1
      email: "user1@example.com"
    - id: 2
      email: "user2@example.com"
```

#### INSERT Operations

```yaml
- id: create_log
  description: "Create audit log"
  type: insert
  sql: "INSERT INTO logs (message, created_at) VALUES ('test', NOW())"
  expected_changes:
    insert: 1
```

#### UPDATE Operations

```yaml
- id: update_status
  description: "Update user status"
  type: update
  sql: "UPDATE users SET status = 'inactive' WHERE id IN (1,2,3)"
  expected_changes:
    update: 3
```

#### DELETE Operations

```yaml
- id: cleanup_logs
  description: "Delete old logs"
  type: delete
  sql: "DELETE FROM logs WHERE created_at < '2025-01-01'"
  expected_changes:
    delete: 100
```

### Template Parameters

Use Go text/template syntax to substitute parameters:

```yaml
params:
  cutoff_date: "2025-01-01"
  user_ids: "1,2,3,4,5"
operations:
  - id: example
    type: select
    sql: |
      SELECT * FROM users
      WHERE created_at >= '{{ .params.cutoff_date }}'
        AND id IN ({{ .params.user_ids }})
```

## Environment Variables

### .env File Support

opsql automatically loads environment variables from a `.env` file in the current directory if it exists. This is the recommended way to manage your configuration.

```bash
# Create your .env file from the example
cp .env.example .env
# Edit .env with your actual values
```

**Note**: The `.env` file is ignored by git to prevent accidental commits of sensitive information.

### Required

- `DATABASE_URL`: Database connection string
  - PostgreSQL: `postgres://user:password@host:port/dbname`
  - MySQL: `mysql://user:password@tcp(host:port)/dbname`

### Optional

- `GITHUB_TOKEN`: GitHub personal access token for PR comments
- `GITHUB_REPOSITORY`: GitHub repository (owner/repo) - auto-detected in GitHub Actions
- `GITHUB_REF`: GitHub reference - auto-detected in GitHub Actions
- `SLACK_WEBHOOK_URL`: Slack incoming webhook URL for notifications

## GitHub Actions Integration

### Example Workflow

```yaml
name: Database Operations
on:
  pull_request:
    paths: ["db/operations/*.yaml"]

jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Install opsql
        run: go install github.com/pyama86/opsql@latest

      - name: Run opsql plan
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
        run: |
          opsql plan \
            --config db/operations/maintenance.yaml \
            --github-repo ${{ github.repository }} \
            --github-pr ${{ github.event.number }}

  apply:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Install opsql
        run: go install github.com/pyama86/opsql@latest

      - name: Run opsql apply
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        run: opsql apply --config db/operations/maintenance.yaml
```

## Common Use Cases

### Bulk Operations with IN Clauses

```yaml
params:
  target_user_ids: "1,2,3,4,5"
operations:
  - id: bulk_update
    description: "Update multiple users"
    type: update
    sql: |
      UPDATE users
      SET status = 'inactive'
      WHERE id IN ({{ .params.target_user_ids }})
    expected_changes:
      update: 5
```

### Data Validation Before Changes

```yaml
operations:
  - id: validate_data
    description: "Validate data before changes"
    type: select
    sql: "SELECT COUNT(*) as cnt FROM users WHERE status = 'pending'"
    expected:
      - cnt: 10

  - id: process_pending
    description: "Process pending users"
    type: update
    sql: "UPDATE users SET status = 'active' WHERE status = 'pending'"
    expected_changes:
      update: 10
```

### Cleanup Operations

```yaml
operations:
  - id: cleanup_orphaned
    description: "Remove orphaned records"
    type: delete
    sql: |
      DELETE FROM user_sessions
      WHERE user_id NOT IN (SELECT id FROM users)
        AND created_at < NOW() - INTERVAL '30 days'
    expected_changes:
      delete: 150
```

## Troubleshooting

### Common Issues

**Q: "DATABASE_URL environment variable is required" error**
A: Set the DATABASE_URL environment variable with your database connection string.

**Q: "connection refused" error**
A: Check your database connection settings:

- Verify the host and port are correct
- Ensure the database is running
- Check network connectivity
- Verify credentials

**Q: Assertion failures**
A: Review your expected values:

- For SELECT: Check that expected rows match actual results exactly
- For DML: Verify expected_changes counts match affected rows

**Q: GitHub comment not posted**
A: Ensure:

- `GITHUB_TOKEN` environment variable is set
- Token has appropriate permissions
- Repository and PR number are correct

**Q: Slack notification not sent**
A: Verify:

- `SLACK_WEBHOOK_URL` is correctly set
- Webhook URL is valid and active
- Network connectivity to Slack

### Debug Mode

For detailed logging, you can examine the JSON output from plan/apply commands:

```bash
opsql plan --config operations.yaml | jq '.'
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run tests: `go test ./...`
6. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.
