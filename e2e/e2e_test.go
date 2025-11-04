package e2e

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
	"github.com/pyama86/opsql/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDatabase はデータベースのセットアップを行う
func setupDatabase(t *testing.T, dsn, dbType string) (*sql.DB, func()) {
	db, err := sql.Open(dbType, dsn)
	require.NoError(t, err, "Failed to connect to database")

	// 接続確認
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	require.NoError(t, err, "Failed to ping database")

	// テスト用テーブルを作成
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INT PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(100),
			status VARCHAR(50)
		)
	`
	_, err = db.ExecContext(context.Background(), createTableSQL)
	require.NoError(t, err, "Failed to create test table")

	cleanup := func() {
		// テーブルをクリーンアップ
		_, _ = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS users")
		_ = db.Close()
	}

	return db, cleanup
}

// insertTestData はテストデータを挿入する
func insertTestData(t *testing.T, db *sql.DB) {
	testData := []struct {
		id     int
		name   string
		email  string
		status string
	}{
		{1, "Alice", "alice@example.com", "active"},
		{2, "Bob", "bob@example.com", "active"},
		{3, "Charlie", "charlie@example.com", "inactive"},
	}

	for _, data := range testData {
		_, err := db.ExecContext(context.Background(),
			"INSERT INTO users (id, name, email, status) VALUES (?, ?, ?, ?)",
			data.id, data.name, data.email, data.status)
		// PostgreSQLの場合はプレースホルダが$1, $2...なので、エラーの場合は再試行
		if err != nil {
			_, err = db.ExecContext(context.Background(),
				"INSERT INTO users (id, name, email, status) VALUES ($1, $2, $3, $4)",
				data.id, data.name, data.email, data.status)
			require.NoError(t, err, "Failed to insert test data")
		}
	}
}

func TestE2E_MySQL(t *testing.T) {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		t.Skip("MYSQL_DSN not set, skipping MySQL E2E test")
	}

	db, cleanup := setupDatabase(t, mysqlDSN, "mysql")
	defer cleanup()

	insertTestData(t, db)

	// データベースアダプタを作成
	dbAdapter, err := database.NewDatabase(mysqlDSN)
	require.NoError(t, err)
	defer func() {
		if err := dbAdapter.Close(); err != nil {
			t.Logf("Warning: failed to close database adapter: %v", err)
		}
	}()

	t.Run("PlanExecutor with SELECT", func(t *testing.T) {
		def := &definition.Definition{
			Version: 1,
			Operations: []definition.Operation{
				{
					ID:          "select_active_users",
					Description: "Select active users",
					Type:        definition.TypeSelect,
					SQL:         "SELECT id, name, email FROM users WHERE status = 'active' ORDER BY id",
					Expected: []map[string]interface{}{
						{"id": int64(1), "name": []byte("Alice"), "email": []byte("alice@example.com")},
						{"id": int64(2), "name": []byte("Bob"), "email": []byte("bob@example.com")},
					},
				},
			},
		}

		planExecutor := executor.NewPlanExecutor(dbAdapter)
		reports, err := planExecutor.Execute(context.Background(), def)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.True(t, reports[0].Pass, "SELECT operation should pass")
	})

	t.Run("PlanExecutor with UPDATE", func(t *testing.T) {
		def := &definition.Definition{
			Version: 1,
			Operations: []definition.Operation{
				{
					ID:          "update_user_status",
					Description: "Update user status",
					Type:        definition.TypeUpdate,
					SQL:         "UPDATE users SET status = 'suspended' WHERE id = 3",
					ExpectedChanges: map[string]int{
						"update": 1,
					},
				},
			},
		}

		planExecutor := executor.NewPlanExecutor(dbAdapter)
		reports, err := planExecutor.Execute(context.Background(), def)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.True(t, reports[0].Pass, "UPDATE operation should pass")

		// ロールバックされているため、実際のデータは変更されていないことを確認
		var status string
		err = db.QueryRowContext(context.Background(), "SELECT status FROM users WHERE id = 3").Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "inactive", status, "Status should not be changed in plan mode")
	})

	t.Run("ApplyExecutor with UPDATE", func(t *testing.T) {
		def := &definition.Definition{
			Version: 1,
			Operations: []definition.Operation{
				{
					ID:          "update_user_status",
					Description: "Update user status",
					Type:        definition.TypeUpdate,
					SQL:         "UPDATE users SET status = 'suspended' WHERE id = 3",
					ExpectedChanges: map[string]int{
						"update": 1,
					},
				},
			},
		}

		applyExecutor := executor.NewApplyExecutor(dbAdapter)
		reports, err := applyExecutor.Execute(context.Background(), def)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.True(t, reports[0].Pass, "UPDATE operation should pass")

		// コミットされているため、実際のデータが変更されていることを確認
		var status string
		err = db.QueryRowContext(context.Background(), "SELECT status FROM users WHERE id = 3").Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "suspended", status, "Status should be changed in apply mode")
	})
}

func TestE2E_PostgreSQL(t *testing.T) {
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		t.Skip("POSTGRES_DSN not set, skipping PostgreSQL E2E test")
	}

	db, cleanup := setupDatabase(t, postgresDSN, "postgres")
	defer cleanup()

	insertTestData(t, db)

	// データベースアダプタを作成
	dbAdapter, err := database.NewDatabase(postgresDSN)
	require.NoError(t, err)
	defer func() {
		if err := dbAdapter.Close(); err != nil {
			t.Logf("Warning: failed to close database adapter: %v", err)
		}
	}()

	t.Run("PlanExecutor with SELECT", func(t *testing.T) {
		def := &definition.Definition{
			Version: 1,
			Operations: []definition.Operation{
				{
					ID:          "select_active_users",
					Description: "Select active users",
					Type:        definition.TypeSelect,
					SQL:         "SELECT id, name, email FROM users WHERE status = 'active' ORDER BY id",
					Expected: []map[string]interface{}{
						{"id": int64(1), "name": "Alice", "email": "alice@example.com"},
						{"id": int64(2), "name": "Bob", "email": "bob@example.com"},
					},
				},
			},
		}

		planExecutor := executor.NewPlanExecutor(dbAdapter)
		reports, err := planExecutor.Execute(context.Background(), def)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.True(t, reports[0].Pass, "SELECT operation should pass")
	})

	t.Run("PlanExecutor with UPDATE", func(t *testing.T) {
		def := &definition.Definition{
			Version: 1,
			Operations: []definition.Operation{
				{
					ID:          "update_user_status",
					Description: "Update user status",
					Type:        definition.TypeUpdate,
					SQL:         "UPDATE users SET status = 'suspended' WHERE id = 3",
					ExpectedChanges: map[string]int{
						"update": 1,
					},
				},
			},
		}

		planExecutor := executor.NewPlanExecutor(dbAdapter)
		reports, err := planExecutor.Execute(context.Background(), def)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.True(t, reports[0].Pass, "UPDATE operation should pass")

		// ロールバックされているため、実際のデータは変更されていないことを確認
		var status string
		err = db.QueryRowContext(context.Background(), "SELECT status FROM users WHERE id = 3").Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "inactive", status, "Status should not be changed in plan mode")
	})

	t.Run("ApplyExecutor with UPDATE", func(t *testing.T) {
		def := &definition.Definition{
			Version: 1,
			Operations: []definition.Operation{
				{
					ID:          "update_user_status",
					Description: "Update user status",
					Type:        definition.TypeUpdate,
					SQL:         "UPDATE users SET status = 'suspended' WHERE id = 3",
					ExpectedChanges: map[string]int{
						"update": 1,
					},
				},
			},
		}

		applyExecutor := executor.NewApplyExecutor(dbAdapter)
		reports, err := applyExecutor.Execute(context.Background(), def)
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.True(t, reports[0].Pass, "UPDATE operation should pass")

		// コミットされているため、実際のデータが変更されていることを確認
		var status string
		err = db.QueryRowContext(context.Background(), "SELECT status FROM users WHERE id = 3").Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "suspended", status, "Status should be changed in apply mode")
	})
}
