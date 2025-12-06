package database

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DB interface {
	QueryRowsContext(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (int64, error)
	BeginTransaction(ctx context.Context) (Transaction, error)
	Close() error
}

type Transaction interface {
	QueryRowsContext(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (int64, error)
	Rollback() error
	Commit() error
}

type Database struct {
	*sqlx.DB
	driver       string
	queryTimeout int // タイムアウト時間（秒）
}

type Tx struct {
	*sqlx.Tx
	driver       string
	queryTimeout int
}

func NewDatabase(dsn string) (DB, error) {
	driver, err := detectDriver(dsn)
	if err != nil {
		return nil, err
	}

	connectionString, err := convertDSN(dsn, driver)
	if err != nil {
		return nil, err
	}

	db, err := sqlx.Connect(driver, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queryTimeout := getQueryTimeout()

	return &Database{
		DB:           db,
		driver:       driver,
		queryTimeout: queryTimeout,
	}, nil
}

// getQueryTimeout は環境変数からクエリタイムアウト値を取得する
func getQueryTimeout() int {
	const defaultTimeout = 30
	timeoutStr := os.Getenv("OPSQL_QUERY_TIMEOUT")
	if timeoutStr == "" {
		return defaultTimeout
	}

	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout <= 0 {
		return defaultTimeout
	}

	return timeout
}

func (d *Database) QueryRowsContext(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := d.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var results []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, err
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func (d *Database) ExecContext(ctx context.Context, query string, args ...interface{}) (int64, error) {
	result, err := d.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return affected, nil
}

func (d *Database) BeginTransaction(ctx context.Context) (Transaction, error) {
	tx, err := d.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}

	// セッションレベルでクエリタイムアウトを設定
	if err := d.setQueryTimeout(ctx, tx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to set query timeout: %w", err)
	}

	return &Tx{
		Tx:           tx,
		driver:       d.driver,
		queryTimeout: d.queryTimeout,
	}, nil
}

// setQueryTimeout はデータベースドライバに応じてクエリタイムアウトを設定する
func (d *Database) setQueryTimeout(ctx context.Context, tx *sqlx.Tx) error {
	switch d.driver {
	case "mysql":
		// MySQLの場合、max_execution_timeを設定（ミリ秒単位）
		timeoutMs := d.queryTimeout * 1000
		query := fmt.Sprintf("SET SESSION max_execution_time = %d", timeoutMs)
		_, err := tx.ExecContext(ctx, query)
		return err
	case "postgres":
		// PostgreSQLの場合、statement_timeoutを設定（ミリ秒単位）
		timeoutMs := d.queryTimeout * 1000
		query := fmt.Sprintf("SET SESSION statement_timeout = %d", timeoutMs)
		_, err := tx.ExecContext(ctx, query)
		return err
	default:
		return nil
	}
}

func (t *Tx) QueryRowsContext(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := t.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var results []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, err
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (int64, error) {
	result, err := t.Tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return affected, nil
}

func (t *Tx) Rollback() error {
	return t.Tx.Rollback()
}

func (t *Tx) Commit() error {
	return t.Tx.Commit()
}

func detectDriver(dsn string) (string, error) {
	dsn = strings.ToLower(dsn)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return "postgres", nil
	}
	if strings.HasPrefix(dsn, "mysql://") || strings.Contains(dsn, "@tcp(") {
		return "mysql", nil
	}
	return "", fmt.Errorf("unsupported database driver in DSN: %s", dsn)
}

func convertDSN(dsn, driver string) (string, error) {
	switch driver {
	case "mysql":
		if strings.HasPrefix(dsn, "mysql://") {
			return strings.TrimPrefix(dsn, "mysql://"), nil
		}
		return dsn, nil
	case "postgres":
		return dsn, nil
	default:
		return dsn, nil
	}
}

func MaskSecret(dsn string) string {
	re := regexp.MustCompile(`://([^:]+):([^@]+)@`)
	return re.ReplaceAllString(dsn, "://$1:***@")
}
