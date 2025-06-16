package database

import (
	"context"
	"fmt"
	"regexp"
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
	driver string
}

type Tx struct {
	*sqlx.Tx
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

	return &Database{
		DB:     db,
		driver: driver,
	}, nil
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

	return &Tx{Tx: tx}, nil
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
