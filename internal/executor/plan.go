package executor

import (
	"context"
	"fmt"

	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
)

type PlanExecutor struct {
	db database.DB
}

func NewPlanExecutor(db database.DB) *PlanExecutor {
	return &PlanExecutor{db: db}
}

func (e *PlanExecutor) Execute(ctx context.Context, def *definition.Definition) ([]definition.Report, error) {
	var reports []definition.Report

	for _, op := range def.Operations {
		report, err := e.executeOperation(ctx, op)
		if err != nil {
			return nil, fmt.Errorf("operation[%s]: %w", op.ID, err)
		}
		reports = append(reports, *report)
	}

	return reports, nil
}

func (e *PlanExecutor) executeOperation(ctx context.Context, op definition.Operation) (*definition.Report, error) {
	switch op.Type {
	case definition.TypeSelect:
		return e.executeSelect(ctx, op)
	case definition.TypeInsert, definition.TypeUpdate, definition.TypeDelete:
		return e.executeDML(ctx, op)
	default:
		return nil, fmt.Errorf("unsupported operation type: %s", op.Type)
	}
}

func (e *PlanExecutor) executeSelect(ctx context.Context, op definition.Operation) (*definition.Report, error) {
	rows, err := e.db.QueryRowsContext(ctx, op.SQL)
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

	return &definition.Report{
		ID:          op.ID,
		Description: op.Description,
		Type:        op.Type,
		Result:      rows,
		Pass:        pass,
		Message:     message,
	}, nil
}

func (e *PlanExecutor) executeDML(ctx context.Context, op definition.Operation) (*definition.Report, error) {
	tx, err := e.db.BeginTransaction(ctx)
	if err != nil {
		return &definition.Report{
			ID:          op.ID,
			Description: op.Description,
			Type:        op.Type,
			Result:      nil,
			Pass:        false,
			Message:     fmt.Sprintf("failed to begin transaction: %v", err),
		}, nil
	}
	defer tx.Rollback()

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

func (e *PlanExecutor) validateSelectResult(actual []map[string]interface{}, expected []map[string]interface{}) (bool, string) {
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

func (e *PlanExecutor) validateDMLResult(actual int64, expected map[string]int, opType string) (bool, string) {
	expectedCount, exists := expected[opType]
	if !exists {
		return false, fmt.Sprintf("no expected count specified for operation type '%s'", opType)
	}

	if actual != int64(expectedCount) {
		return false, fmt.Sprintf("affected rows mismatch: expected %d, got %d", expectedCount, actual)
	}

	return true, "assertion passed"
}
