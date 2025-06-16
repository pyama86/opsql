package executor

import (
	"context"
	"fmt"
	"os"

	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
)

type ApplyExecutor struct {
	*BaseExecutor
}

func NewApplyExecutor(db database.DB) *ApplyExecutor {
	return &ApplyExecutor{
		BaseExecutor: NewBaseExecutor(db),
	}
}

func (e *ApplyExecutor) Execute(ctx context.Context, def *definition.Definition) ([]definition.Report, error) {
	tx, err := e.db.BeginTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	var reports []definition.Report

	for _, op := range def.Operations {
		report, err := e.executeOperation(ctx, tx, op)
		if report != nil {
			reports = append(reports, *report)
			if !report.Pass {
				fmt.Fprintf(os.Stderr, "Operation[%s] failed: %s\n", report.ID, report.Message)
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return reports, fmt.Errorf("operation[%s]: %w", op.ID, err)
		}

		if !report.Pass {
			_ = tx.Rollback()
			fmt.Fprintf(os.Stderr, "Assertion failed for operation[%s]: %s\n", op.ID, report.Message)
			os.Exit(1)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return reports, nil
}
