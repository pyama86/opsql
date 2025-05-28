package executor

import (
	"context"
	"fmt"
	"os"

	"github.com/pyama86/opsql/internal/database"
	"github.com/pyama86/opsql/internal/definition"
)

type PlanExecutor struct {
	*BaseExecutor
}

func NewPlanExecutor(db database.DB) *PlanExecutor {
	return &PlanExecutor{
		BaseExecutor: NewBaseExecutor(db),
	}
}

func (e *PlanExecutor) Execute(ctx context.Context, def *definition.Definition) ([]definition.Report, error) {
	tx, err := e.db.BeginTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

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
			return reports, fmt.Errorf("operation[%s]: %w", op.ID, err)
		}
	}

	return reports, nil
}
