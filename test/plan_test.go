package test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
	"github.com/pyama86/opsql/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockDatabase struct {
	db   *sql.DB
	mock sqlmock.Sqlmock
}

func (m *MockDatabase) QueryRowsContext(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func (m *MockDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (int64, error) {
	result, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return affected, err
}

func (m *MockDatabase) BeginTransaction(ctx context.Context) (database.Transaction, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &MockTransaction{tx: tx}, nil
}

func (m *MockDatabase) Close() error {
	return m.db.Close()
}

type MockTransaction struct {
	tx *sql.Tx
}

func (m *MockTransaction) ExecContext(ctx context.Context, query string, args ...interface{}) (int64, error) {
	result, err := m.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return affected, err
}

func (m *MockTransaction) Rollback() error {
	return m.tx.Rollback()
}

func (m *MockTransaction) Commit() error {
	return m.tx.Commit()
}

func TestPlanExecutor_Execute(t *testing.T) {
	tests := []struct {
		name       string
		definition *definition.Definition
		setupMock  func(sqlmock.Sqlmock)
		wantPass   bool
		wantError  bool
	}{
		{
			name: "successful SELECT with IN clause",
			definition: &definition.Definition{
				Version: 1,
				Operations: []definition.Operation{
					{
						ID:          "check_users",
						Description: "Check specific users",
						Type:        definition.TypeSelect,
						SQL:         "SELECT id, email FROM users WHERE id IN (1,2,3)",
						Expected: []map[string]interface{}{
							{"id": int64(1), "email": "user1@example.com"},
							{"id": int64(2), "email": "user2@example.com"},
						},
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email"}).
					AddRow(1, "user1@example.com").
					AddRow(2, "user2@example.com")
				mock.ExpectQuery("SELECT id, email FROM users WHERE id IN \\(1,2,3\\)").WillReturnRows(rows)
			},
			wantPass:  true,
			wantError: false,
		},
		{
			name: "successful DELETE with IN clause",
			definition: &definition.Definition{
				Version: 1,
				Operations: []definition.Operation{
					{
						ID:          "delete_user_logs",
						Description: "Delete logs for specific users",
						Type:        definition.TypeDelete,
						SQL:         "DELETE FROM logs WHERE user_id IN (1,2,3)",
						ExpectedChanges: map[string]int{
							"delete": 15,
						},
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM logs WHERE user_id IN \\(1,2,3\\)").
					WillReturnResult(sqlmock.NewResult(0, 15))
				mock.ExpectRollback()
			},
			wantPass:  true,
			wantError: false,
		},
		{
			name: "successful UPDATE with IN clause",
			definition: &definition.Definition{
				Version: 1,
				Operations: []definition.Operation{
					{
						ID:          "update_users",
						Description: "Update specific users",
						Type:        definition.TypeUpdate,
						SQL:         "UPDATE users SET status = 'inactive' WHERE id IN (1,2,3)",
						ExpectedChanges: map[string]int{
							"update": 3,
						},
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE users SET status = 'inactive' WHERE id IN \\(1,2,3\\)").
					WillReturnResult(sqlmock.NewResult(0, 3))
				mock.ExpectRollback()
			},
			wantPass:  true,
			wantError: false,
		},
		{
			name: "failed assertion - wrong affected rows",
			definition: &definition.Definition{
				Version: 1,
				Operations: []definition.Operation{
					{
						ID:          "delete_fail",
						Description: "Delete with wrong expectation",
						Type:        definition.TypeDelete,
						SQL:         "DELETE FROM logs WHERE user_id IN (1,2,3)",
						ExpectedChanges: map[string]int{
							"delete": 20,
						},
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM logs WHERE user_id IN \\(1,2,3\\)").
					WillReturnResult(sqlmock.NewResult(0, 15))
				mock.ExpectRollback()
			},
			wantPass:  false,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.setupMock(mock)

			mockDB := &MockDatabase{db: db, mock: mock}
			planExecutor := executor.NewPlanExecutor(mockDB)

			reports, err := planExecutor.Execute(context.Background(), tt.definition)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, reports, 1)

			report := reports[0]
			assert.Equal(t, tt.wantPass, report.Pass)
			assert.Equal(t, tt.definition.Operations[0].ID, report.ID)
			assert.Equal(t, tt.definition.Operations[0].Type, report.Type)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
