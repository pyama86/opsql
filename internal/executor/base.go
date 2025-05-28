package executor

import (
	"context"
	"fmt"

	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
)

type BaseExecutor struct {
	db database.DB
}

func NewBaseExecutor(db database.DB) *BaseExecutor {
	return &BaseExecutor{db: db}
}

func (e *BaseExecutor) executeOperation(ctx context.Context, tx database.Transaction, op definition.Operation) (*definition.Report, error) {
	switch op.Type {
	case definition.TypeSelect:
		return e.executeSelect(ctx, tx, op)
	case definition.TypeInsert, definition.TypeUpdate, definition.TypeDelete:
		return e.executeDML(ctx, tx, op)
	default:
		return nil, fmt.Errorf("unsupported operation type: %s", op.Type)
	}
}

func (e *BaseExecutor) executeSelect(ctx context.Context, tx database.Transaction, op definition.Operation) (*definition.Report, error) {
	rows, err := tx.QueryRowsContext(ctx, op.SQL)
	if err != nil {
		return &definition.Report{
			ID:          op.ID,
			Description: op.Description,
			Type:        op.Type,
			Result:      nil,
			Pass:        false,
			Message:     fmt.Sprintf("query failed: %v", err),
		}, nil
	}

	pass, message := e.validateSelectResult(rows, op.Expected)
	if !pass {
		err = fmt.Errorf("assertion failed: %s", message)
	}

	return &definition.Report{
		ID:          op.ID,
		Description: op.Description,
		Type:        op.Type,
		Result:      rows,
		Pass:        pass,
		Message:     message,
	}, err
}

func (e *BaseExecutor) executeDML(ctx context.Context, tx database.Transaction, op definition.Operation) (*definition.Report, error) {
	affected, err := tx.ExecContext(ctx, op.SQL)
	if err != nil {
		return &definition.Report{
			ID:          op.ID,
			Description: op.Description,
			Type:        op.Type,
			Result:      nil,
			Pass:        false,
			Message:     fmt.Sprintf("execution failed: %v", err),
		}, nil
	}

	pass, message := e.validateDMLResult(affected, op.ExpectedChanges, op.Type)

	return &definition.Report{
		ID:          op.ID,
		Description: op.Description,
		Type:        op.Type,
		Result:      affected,
		Pass:        pass,
		Message:     message,
	}, nil
}

func (e *BaseExecutor) validateSelectResult(actual []map[string]interface{}, expected []map[string]interface{}) (bool, string) {
	if len(actual) != len(expected) {
		return false, fmt.Sprintf("row count mismatch: expected %d, got %d", len(expected), len(actual))
	}

	for i, expectedRow := range expected {
		if i >= len(actual) {
			return false, fmt.Sprintf("missing row at index %d", i)
		}

		actualRow := actual[i]
		for key, expectedValue := range expectedRow {
			actualValue, exists := actualRow[key]
			if !exists {
				return false, fmt.Sprintf("missing column '%s' in row %d", key, i)
			}

			if !compareValues(actualValue, expectedValue) {
				return false, fmt.Sprintf("value mismatch in row %d, column '%s': expected %v, got %v", i, key, expectedValue, actualValue)
			}
		}
	}

	return true, "assertion passed"
}

func (e *BaseExecutor) validateDMLResult(actual int64, expected map[string]int, opType string) (bool, string) {
	expectedCount, exists := expected[opType]
	if !exists {
		return false, fmt.Sprintf("no expected count specified for operation type '%s'", opType)
	}

	if actual != int64(expectedCount) {
		return false, fmt.Sprintf("affected rows mismatch: expected %d, got %d", expectedCount, actual)
	}

	return true, "assertion passed"
}
